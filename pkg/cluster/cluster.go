package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/docker/docker/client"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/tilt-dev/ctlptl/internal/socat"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/registry"
	"github.com/tilt-dev/localregistry-go"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	// Client auth plugins! They will auto-init if we import them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

const clusterSpecConfigMap = "ctlptl-cluster-spec"

var typeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "Cluster"}
var listTypeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "ClusterList"}
var groupResource = schema.GroupResource{Group: "ctlptl.dev", Resource: "clusters"}

// Due to the way the Kubernetes apiserver works, there's no easy way to
// distinguish between "server is taking a long time to respond because it's
// gone" and "server is taking a long time to respond because it has a slow auth
// plugin".
//
// So our health check timeout is a bit longer than we'd like.
// Fortunately, ctlptl is mostly used for local clusters.
const healthCheckTimeout = 3 * time.Second

const waitAfterCreateTimeout = 2 * time.Minute

func TypeMeta() api.TypeMeta {
	return typeMeta
}
func ListTypeMeta() api.TypeMeta {
	return listTypeMeta
}

type configLoader func() (clientcmdapi.Config, error)

type registryController interface {
	Apply(ctx context.Context, r *api.Registry) (*api.Registry, error)
	List(ctx context.Context, options registry.ListOptions) (*api.RegistryList, error)
}

type clientLoader func(*rest.Config) (kubernetes.Interface, error)

type socatController interface {
	ConnectRemoteDockerPort(ctx context.Context, port int) error
}

type Controller struct {
	iostreams              genericclioptions.IOStreams
	config                 clientcmdapi.Config
	clients                map[string]kubernetes.Interface
	admins                 map[Product]Admin
	dockerClient           dockerClient
	dmachine               *dockerMachine
	configLoader           configLoader
	configWriter           configWriter
	registryCtl            registryController
	clientLoader           clientLoader
	socat                  socatController
	waitAfterCreateTimeout time.Duration

	// TODO(nick): I deeply regret making this struct use goroutines. It makes
	// everything so much more complex.
	//
	// We should try to split this up into two structs - the part that needs
	// concurrency for performance, and the part that is fine being
	// single-threaded.
	mu sync.Mutex
}

func DefaultController(iostreams genericclioptions.IOStreams) (*Controller, error) {
	configLoader := configLoader(func() (clientcmdapi.Config, error) {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

		overrides := &clientcmd.ConfigOverrides{}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
		return loader.RawConfig()
	})

	configWriter := kubeconfigWriter{iostreams: iostreams}

	clientLoader := clientLoader(func(restConfig *rest.Config) (kubernetes.Interface, error) {
		return kubernetes.NewForConfig(restConfig)
	})

	config, err := configLoader()
	if err != nil {
		return nil, err
	}

	return &Controller{
		iostreams:              iostreams,
		config:                 config,
		configWriter:           configWriter,
		clients:                make(map[string]kubernetes.Interface),
		admins:                 make(map[Product]Admin),
		configLoader:           configLoader,
		clientLoader:           clientLoader,
		waitAfterCreateTimeout: waitAfterCreateTimeout,
	}, nil
}

func (c *Controller) getSocatController(ctx context.Context) (socatController, error) {
	dcli, err := c.getDockerClient(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.socat == nil {
		c.socat = socat.NewController(dcli)
	}

	return c.socat, nil
}

func (c *Controller) getDockerClient(ctx context.Context) (dockerClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.dockerClient != nil {
		return c.dockerClient, nil
	}

	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	client.NegotiateAPIVersion(ctx)
	c.dockerClient = client
	return client, nil
}

func (c *Controller) machine(ctx context.Context, name string, product Product) (Machine, error) {
	dockerClient, err := c.getDockerClient(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	switch product {
	case ProductDockerDesktop, ProductKIND, ProductK3D:
		if c.dmachine == nil {
			machine, err := NewDockerMachine(ctx, dockerClient, c.iostreams.ErrOut)
			if err != nil {
				return nil, err
			}
			c.dmachine = machine
		}
		return c.dmachine, nil

	case ProductMinikube:
		if c.dmachine == nil {
			machine, err := NewDockerMachine(ctx, dockerClient, c.iostreams.ErrOut)
			if err != nil {
				return nil, err
			}
			c.dmachine = machine
		}
		return newMinikubeMachine(name, c.dmachine), nil
	}

	return unknownMachine{product: product}, nil
}

func (c *Controller) registryController(ctx context.Context) (registryController, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := c.registryCtl
	if result == nil {
		var err error
		result, err = registry.DefaultController(ctx, c.iostreams)
		if err != nil {
			return nil, err
		}
		c.registryCtl = result
	}
	return result, nil
}

// A cluster admin provides the basic start/stop functionality of a cluster,
// independent of the configuration of the machine it's running on.
func (c *Controller) admin(ctx context.Context, product Product) (Admin, error) {
	dockerClient, err := c.getDockerClient(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	admin, ok := c.admins[product]
	if ok {
		return admin, nil
	}

	switch product {
	case ProductDockerDesktop:
		admin = newDockerDesktopAdmin()
	case ProductKIND:
		admin = newKindAdmin(c.iostreams)
	case ProductMinikube:
		admin = newMinikubeAdmin(c.iostreams, dockerClient)
	}

	if product == "" {
		return nil, fmt.Errorf("you must specify a 'product' field in your cluster config")
	}
	if admin == nil {
		return nil, fmt.Errorf("ctlptl doesn't know how to set up clusters for product: %s", product)
	}
	c.admins[product] = admin
	return admin, nil
}

func (c *Controller) configCopy() *clientcmdapi.Config {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.config.DeepCopy()
}

// Gets the port of the current API server.
func (c *Controller) currentAPIServerPort() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	current := c.config.CurrentContext
	context, ok := c.config.Contexts[current]
	if !ok {
		return 0
	}

	cluster, ok := c.config.Clusters[context.Cluster]
	if !ok {
		return 0
	}

	parts := strings.Split(cluster.Server, ":")
	port, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return 0
	}
	return port
}

func (c *Controller) configCurrent() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.config.CurrentContext
}

func (c *Controller) client(name string) (kubernetes.Interface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	client, ok := c.clients[name]
	if ok {
		return client, nil
	}

	restConfig, err := clientcmd.NewDefaultClientConfig(
		c.config, &clientcmd.ConfigOverrides{CurrentContext: name}).ClientConfig()
	if err != nil {
		return nil, err
	}

	client, err = c.clientLoader(restConfig)
	if err != nil {
		return nil, err
	}
	c.clients[name] = client
	return client, nil
}

func (c *Controller) populateCreationTimestamp(ctx context.Context, cluster *api.Cluster, client kubernetes.Interface) error {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	minTime := metav1.Time{}
	for _, node := range nodes.Items {
		cTime := node.CreationTimestamp
		if minTime.Time.IsZero() || cTime.Time.Before(minTime.Time) {
			minTime = cTime
		}
	}

	cluster.Status.CreationTimestamp = minTime

	return nil
}

func (c *Controller) populateLocalRegistryHosting(ctx context.Context, cluster *api.Cluster, client kubernetes.Interface) error {
	hosting, err := localregistry.Discover(ctx, client.CoreV1())
	if err != nil {
		return err
	}

	cluster.Status.LocalRegistryHosting = &hosting

	if hosting.Host == "" {
		return nil
	}

	// Let's try to find the registry corresponding to this cluster.
	var port int
	_, err = fmt.Sscanf(hosting.Host, "localhost:%d", &port)
	if err != nil || port == 0 {
		return err
	}

	registryCtl, err := c.registryController(ctx)
	if err != nil {
		return err
	}

	registryList, err := registryCtl.List(ctx, registry.ListOptions{FieldSelector: fmt.Sprintf("port=%d", port)})
	if err != nil {
		return err
	}

	if len(registryList.Items) == 0 {
		return nil
	}

	cluster.Registry = registryList.Items[0].Name

	return nil
}

func (c *Controller) populateMachineStatus(ctx context.Context, cluster *api.Cluster) error {
	machine, err := c.machine(ctx, cluster.Name, Product(cluster.Product))
	if err != nil {
		return err
	}

	cpu, err := machine.CPUs(ctx)
	if err != nil {
		return err
	}
	cluster.Status.CPUs = cpu
	return nil
}

func (c *Controller) populateClusterSpec(ctx context.Context, cluster *api.Cluster, client kubernetes.Interface) error {
	cMap, err := client.CoreV1().ConfigMaps("kube-public").Get(ctx, clusterSpecConfigMap, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsForbidden(err) {
			return nil
		}
		return err
	}

	spec := api.Cluster{}
	err = yaml.Unmarshal([]byte(cMap.Data["cluster.v1alpha1"]), &spec)
	if err != nil {
		return err
	}

	cluster.KubernetesVersion = spec.KubernetesVersion
	cluster.MinCPUs = spec.MinCPUs
	cluster.KindV1Alpha4Cluster = spec.KindV1Alpha4Cluster
	return nil
}

// If you have dead clusters in your kubeconfig, it's common for the requests to
// hang indefinitely. So we do a quick health check with a short timeout.
func (c *Controller) healthCheckCluster(ctx context.Context, client kubernetes.Interface) (*version.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	return c.serverVersion(ctx, client)
}

// A fork of DiscoveryClient ServerVersion that obeys Context timeouts.
func (c *Controller) serverVersion(ctx context.Context, client kubernetes.Interface) (*version.Info, error) {
	restClient := client.Discovery().RESTClient()
	if restClient == nil {
		return client.Discovery().ServerVersion()
	}

	body, err := restClient.Get().AbsPath("/version").Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	var info version.Info
	err = json.Unmarshal(body, &info)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the server version: %v", err)
	}
	return &info, nil
}

func (c *Controller) populateCluster(ctx context.Context, cluster *api.Cluster) {
	name := cluster.Name
	client, err := c.client(cluster.Name)
	if err != nil {
		klog.V(4).Infof("WARNING: creating cluster %s client: %v\n", name, err)
		return
	}
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(ctx)

	wg.Add(1)
	go func() {
		defer wg.Done()

		v, err := c.healthCheckCluster(ctx, client)
		if err != nil {
			// Cancel all other fetching.
			cancel()
			return
		}

		cluster.Status.KubernetesVersion = v.GitVersion
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := c.populateCreationTimestamp(ctx, cluster, client)
		if err != nil {
			klog.V(4).Infof("WARNING: reading cluster %s creation time: %v\n", name, err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := c.populateLocalRegistryHosting(ctx, cluster, client)
		if err != nil {
			klog.V(4).Infof("WARNING: reading cluster %s registry: %v\n", name, err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := c.populateMachineStatus(ctx, cluster)
		if err != nil {
			klog.V(4).Infof("WARNING: reading cluster %s machine: %v\n", name, err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := c.populateClusterSpec(ctx, cluster, client)
		if err != nil {
			klog.V(4).Infof("WARNING: reading cluster %s spec: %v\n", name, err)
		}
	}()

	wg.Wait()

	cluster.Status.Current = c.configCurrent() == cluster.Name
}

func FillDefaults(cluster *api.Cluster) {
	// If the name is in the Kind config, but not in the main config,
	// lift it up to the main config.
	if cluster.KindV1Alpha4Cluster != nil && cluster.Name == "" && cluster.KindV1Alpha4Cluster.Name != "" {
		cluster.Name = fmt.Sprintf("kind-%s", cluster.KindV1Alpha4Cluster.Name)
	}

	// Create a default name if one isn't in the YAML.
	// The default name is determined by the underlying product.
	if cluster.Name == "" {
		cluster.Name = Product(cluster.Product).DefaultClusterName()
	}

	// Override the Kind config if necessary.
	if cluster.KindV1Alpha4Cluster != nil {
		cluster.KindV1Alpha4Cluster.Name = strings.TrimPrefix(cluster.Name, "kind-")
	}
}

// TODO(nick): Add more registry-supporting clusters.
func supportsRegistry(product Product) bool {
	return product == ProductKIND || product == ProductMinikube
}

func supportsKubernetesVersion(product Product, version string) bool {
	return product == ProductKIND || product == ProductMinikube
}

func (c *Controller) canReconcileK8sVersion(ctx context.Context, desired, existing *api.Cluster) bool {
	if desired.KubernetesVersion == "" {
		return true
	}

	if desired.KubernetesVersion == existing.Status.KubernetesVersion {
		return true
	}

	// On KIND, it's ok if the patch doesn't match.
	if Product(desired.Product) == ProductKIND {
		dv, err := semver.ParseTolerant(desired.KubernetesVersion)
		if err != nil {
			return false
		}
		ev, err := semver.ParseTolerant(existing.Status.KubernetesVersion)
		if err != nil {
			return false
		}
		return dv.Major == ev.Major && dv.Minor == ev.Minor
	}

	return false
}

func (c *Controller) deleteIfIrreconcilable(ctx context.Context, desired, existing *api.Cluster) error {
	if existing.Name == "" {
		// Nothing to delete
		return nil
	}

	needsDelete := false
	if existing.Product != "" && existing.Product != desired.Product {
		_, _ = fmt.Fprintf(c.iostreams.ErrOut, "Deleting cluster %s to change admin from %s to %s\n",
			desired.Name, existing.Product, desired.Product)
		needsDelete = true
	} else if desired.Registry != "" && desired.Registry != existing.Registry {
		// TODO(nick): Ideally, we should be able to patch a cluster
		// with a registry, but it gets a little hairy.
		_, _ = fmt.Fprintf(c.iostreams.ErrOut, "Deleting cluster %s to initialize with registry %s\n",
			desired.Name, desired.Registry)
		needsDelete = true
	} else if !c.canReconcileK8sVersion(ctx, desired, existing) {
		_, _ = fmt.Fprintf(c.iostreams.ErrOut,
			"Deleting cluster %s because desired Kubernetes version (%s) does not match current (%s)\n",
			desired.Name, desired.KubernetesVersion, existing.Status.KubernetesVersion)
		needsDelete = true
	} else if desired.KindV1Alpha4Cluster != nil && !cmp.Equal(existing.KindV1Alpha4Cluster, desired.KindV1Alpha4Cluster) {
		_, _ = fmt.Fprintf(c.iostreams.ErrOut,
			"Deleting cluster %s because desired Kind config does not match current.\nCluster config diff: %s\n",
			desired.Name, cmp.Diff(existing.KindV1Alpha4Cluster, desired.KindV1Alpha4Cluster))
		needsDelete = true
	}

	if !needsDelete {
		return nil
	}

	err := c.Delete(ctx, desired.Name)
	if err != nil {
		return err
	}
	*existing = api.Cluster{}
	return nil
}

// Checks if a registry exists with the given name, and creates one if it doesn't.
func (c *Controller) ensureRegistryExists(ctx context.Context, name string) (*api.Registry, error) {
	regCtl, err := c.registryController(ctx)
	if err != nil {
		return nil, err
	}

	return regCtl.Apply(ctx, &api.Registry{
		TypeMeta: registry.TypeMeta(),
		Name:     name,
	})
}

// Compare the desired cluster against the existing cluster, and reconcile
// the two to match.
func (c *Controller) Apply(ctx context.Context, desired *api.Cluster) (*api.Cluster, error) {
	if desired.Product == "" {
		return nil, fmt.Errorf("product field must be non-empty")
	}
	if desired.Registry != "" && !supportsRegistry(Product(desired.Product)) {
		return nil, fmt.Errorf("product %s does not support a registry", desired.Product)
	}
	if desired.KubernetesVersion != "" && !supportsKubernetesVersion(Product(desired.Product), desired.KubernetesVersion) {
		return nil, fmt.Errorf("product %s does not support a custom Kubernetes version", desired.Product)
	}
	if desired.KindV1Alpha4Cluster != nil && Product(desired.Product) != ProductKIND {
		return nil, fmt.Errorf("kind config may only be set on clusters with product: kind. Actual product: %s", desired.Product)
	}

	FillDefaults(desired)

	// Fetch the machine driver for this product and cluster name,
	// and use it to apply the constraints to the underlying VM.
	machine, err := c.machine(ctx, desired.Name, Product(desired.Product))
	if err != nil {
		return nil, err
	}

	// First, we have to make sure the machine driver has started, so that we can
	// query it at all for the existing configuration.
	err = machine.EnsureExists(ctx)
	if err != nil {
		return nil, err
	}

	existingCluster, err := c.Get(ctx, desired.Name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if existingCluster == nil {
		existingCluster = &api.Cluster{}
	}

	// If we can't reconcile the two clusters, delete it now.
	// TODO(nick): Check for a --force flag, and only delete the cluster
	// if there's a --force.
	err = c.deleteIfIrreconcilable(ctx, desired, existingCluster)
	if err != nil {
		return nil, err
	}

	// Fetch the admin driver for this product, for setting up the cluster on top of
	// the machine.
	admin, err := c.admin(ctx, Product(desired.Product))
	if err != nil {
		return nil, err
	}

	existingStatus := existingCluster.Status
	needsRestart := existingStatus.CreationTimestamp.Time.IsZero() ||
		existingStatus.CPUs < desired.MinCPUs
	if needsRestart {
		err := machine.Restart(ctx, desired, existingCluster)
		if err != nil {
			return nil, err
		}
	}

	var reg *api.Registry
	if desired.Registry != "" {
		reg, err = c.ensureRegistryExists(ctx, desired.Registry)
		if err != nil {
			return nil, err
		}
	}

	// Configure the cluster to match what we want.
	needsCreate := existingStatus.CreationTimestamp.Time.IsZero() ||
		desired.Name != existingCluster.Name ||
		desired.Product != existingCluster.Product
	if needsCreate {
		err := admin.Create(ctx, desired, reg)
		if err != nil {
			return nil, err
		}

		err = c.waitForContextCreate(ctx, desired)
		if err != nil {
			return nil, err
		}
	}

	// Update the kubectl context to match this cluster.
	err = c.configWriter.SetContext(desired.Name)
	if err != nil {
		return nil, fmt.Errorf("switching to cluster context %s: %v", desired.Name, err)
	}

	err = c.reloadConfigs()
	if err != nil {
		return nil, err
	}

	if needsCreate {
		// If the cluster apiserver is in a remote docker cluster,
		// set up a portforwarder.
		err := c.maybeCreateForwarderForCurrentCluster(ctx)
		if err != nil {
			return nil, err
		}

		err = c.waitForHealthCheckAfterCreate(ctx, desired)
		if err != nil {
			return nil, err
		}

		err = c.writeClusterSpec(ctx, desired)
		if err != nil {
			return nil, errors.Wrap(err, "configuring cluster")
		}

		if desired.Registry != "" {
			err = c.createRegistryHosting(ctx, admin, desired, reg)
			if err != nil {
				return nil, errors.Wrap(err, "configuring cluster registry")
			}
		}
	}

	return c.Get(ctx, desired.Name)
}

// Writes the cluster spec to the cluster itself, so
// we can read it later to determine how the cluster was initialized.
func (c *Controller) writeClusterSpec(ctx context.Context, cluster *api.Cluster) error {
	client, err := c.client(cluster.Name)
	if err != nil {
		return err
	}

	specOnly := cluster.DeepCopy()
	specOnly.Status = api.ClusterStatus{}
	data, err := yaml.Marshal(specOnly)
	if err != nil {
		return err
	}

	err = client.CoreV1().ConfigMaps("kube-public").Delete(ctx, clusterSpecConfigMap, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	_, err = client.CoreV1().ConfigMaps("kube-public").Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterSpecConfigMap,
			Namespace: "kube-public",
		},
		Data: map[string]string{"cluster.v1alpha1": string(data)},
	}, metav1.CreateOptions{})
	return err
}

// Create a configmap on the cluster, so that other tools know that a registry
// has been configured.
func (c *Controller) createRegistryHosting(ctx context.Context, admin Admin, cluster *api.Cluster, reg *api.Registry) error {
	hosting, err := admin.LocalRegistryHosting(ctx, cluster, reg)
	if err != nil {
		return err
	}
	if hosting == nil {
		return nil
	}

	client, err := c.client(cluster.Name)
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(hosting)
	if err != nil {
		return err
	}

	_, err = client.CoreV1().ConfigMaps("kube-public").Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "local-registry-hosting",
			Namespace: "kube-public",
		},
		Data: map[string]string{"localRegistryHosting.v1": string(data)},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(c.iostreams.ErrOut, " 🔌 Connected cluster %s to registry %s at %s\n", cluster.Name, reg.Name, hosting.Host)
	_, _ = fmt.Fprintf(c.iostreams.ErrOut, " 👐 Push images to the cluster like 'docker push %s/alpine'\n", hosting.Host)

	return nil
}

func (c *Controller) Delete(ctx context.Context, name string) error {
	existing, err := c.Get(ctx, name)
	if err != nil {
		return err
	}

	admin, err := c.admin(ctx, Product(existing.Product))
	if err != nil {
		return err
	}

	err = admin.Delete(ctx, existing)
	if err != nil {
		return err
	}

	err = c.reloadConfigs()
	if err != nil {
		return err
	}

	// If the context is still in the configs, delete it.
	_, ok := c.configCopy().Contexts[existing.Name]
	if ok {
		return c.configWriter.DeleteContext(existing.Name)
	}
	return nil
}

func (c *Controller) reloadConfigs() error {
	config, err := c.configLoader()
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.config = config
	c.clients = make(map[string]kubernetes.Interface)
	return nil
}

func (c *Controller) Current(ctx context.Context) (*api.Cluster, error) {
	current := c.configCurrent()
	if current == "" {
		return nil, fmt.Errorf("no cluster selected in kubeconfig")
	}
	return c.Get(ctx, current)
}

func (c *Controller) Get(ctx context.Context, name string) (*api.Cluster, error) {
	config := c.configCopy()
	ct, ok := config.Contexts[name]
	if !ok {
		return nil, apierrors.NewNotFound(groupResource, name)
	}
	cluster := &api.Cluster{
		TypeMeta: typeMeta,
		Name:     name,
		Product:  productFromContext(ct, config.Clusters[ct.Cluster]).String(),
	}
	c.populateCluster(ctx, cluster)

	return cluster, nil
}

func (c *Controller) List(ctx context.Context, options ListOptions) (*api.ClusterList, error) {
	selector, err := fields.ParseSelector(options.FieldSelector)
	if err != nil {
		return nil, err
	}

	config := c.configCopy()
	names := make([]string, 0, len(c.config.Contexts))
	for name := range config.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)

	// Listing all clusters can take a long time, so parallelize it.
	all := make([]*api.Cluster, len(names))
	g, ctx := errgroup.WithContext(ctx)

	for i, name := range names {
		ct := c.config.Contexts[name]
		name := name
		i := i
		g.Go(func() error {
			cluster := &api.Cluster{
				TypeMeta: typeMeta,
				Name:     name,
				Product:  productFromContext(ct, config.Clusters[ct.Cluster]).String(),
			}
			if !selector.Matches((*clusterFields)(cluster)) {
				return nil
			}
			c.populateCluster(ctx, cluster)
			all[i] = cluster
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	result := []api.Cluster{}
	for _, c := range all {
		if c == nil {
			continue
		}
		result = append(result, *c)
	}

	return &api.ClusterList{
		TypeMeta: listTypeMeta,
		Items:    result,
	}, nil
}

func isLocalDockerHost(dockerHost string) bool {
	return dockerHost == "" ||

		// Check all the "standard" docker localhosts.
		// https://github.com/docker/cli/blob/a32cd16160f1b41c1c4ae7bee4dac929d1484e59/opts/hosts.go#L22
		strings.HasPrefix(dockerHost, "tcp://localhost:") ||
		strings.HasPrefix(dockerHost, "tcp://127.0.0.1:") ||

		// https://github.com/moby/moby/blob/master/client/client_windows.go#L4
		strings.HasPrefix(dockerHost, "npipe:") ||

		// https://github.com/moby/moby/blob/master/client/client_unix.go#L6
		strings.HasPrefix(dockerHost, "unix:")
}

// If the current cluster is on a remote docker instance,
// we need a port-forwarder to connect it.
func (c *Controller) maybeCreateForwarderForCurrentCluster(ctx context.Context) error {
	dockerHost := os.Getenv("DOCKER_HOST")
	if isLocalDockerHost(dockerHost) {
		return nil
	}

	port := c.currentAPIServerPort()
	if port == 0 {
		return nil
	}

	socat, err := c.getSocatController(ctx)
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(c.iostreams.ErrOut, " 🎮 Env DOCKER_HOST set. Assuming remote Docker and forwarding apiserver to localhost:%d\n", port)
	return socat.ConnectRemoteDockerPort(ctx, port)
}

// Docker-Desktop may be slow to write the kubernetes context
// back to the config, so we have to wait until it appears.
func (c *Controller) waitForContextCreate(ctx context.Context, cluster *api.Cluster) error {
	refreshAndCheckOK := func() error {
		err := c.reloadConfigs()
		if err != nil {
			return err
		}
		_, err = c.client(cluster.Name)
		if err != nil {
			return err
		}
		return nil
	}

	err := refreshAndCheckOK()
	if err == nil {
		return nil
	}

	_, _ = fmt.Fprintf(c.iostreams.ErrOut, "Waiting %s for cluster %q to create kubectl context...\n",
		duration.ShortHumanDuration(c.waitAfterCreateTimeout), cluster.Name)
	var lastErr error
	err = wait.PollImmediate(time.Second, c.waitAfterCreateTimeout, func() (bool, error) {
		err := refreshAndCheckOK()
		lastErr = err
		isSuccess := err == nil
		return isSuccess, nil
	})
	if err != nil {
		return fmt.Errorf("kubernetes context never created: %v", lastErr)
	}
	return nil
}

// Our cluster creation tools aren't super trustworthy.
//
// After the cluster is created, we poll the kubeconfig until
// the cluster context has been created and the cluster becomes healthy.
//
// https://github.com/tilt-dev/ctlptl/issues/87
// https://github.com/tilt-dev/ctlptl/issues/131
func (c *Controller) waitForHealthCheckAfterCreate(ctx context.Context, cluster *api.Cluster) error {
	checkOK := func() error {
		client, err := c.client(cluster.Name)
		if err != nil {
			return err
		}

		// quick apiserver health check.
		_, err = c.healthCheckCluster(ctx, client)
		if err != nil {
			return err
		}

		// make sure the kube-public namespace exists,
		// because this is where ctlptl writes its configs.
		_, err = client.CoreV1().Namespaces().Get(ctx, "kube-public", metav1.GetOptions{})
		if err != nil {
			return err
		}

		return nil
	}

	// If the tool properly waited for the cluster to init,
	// return immediately.
	err := checkOK()
	if err == nil {
		return nil
	}

	_, _ = fmt.Fprintf(c.iostreams.ErrOut, "Waiting %s for Kubernetes cluster %q to start...\n",
		duration.ShortHumanDuration(c.waitAfterCreateTimeout), cluster.Name)
	var lastErr error
	err = wait.PollImmediate(time.Second, c.waitAfterCreateTimeout, func() (bool, error) {
		err := checkOK()
		lastErr = err
		isSuccess := err == nil
		return isSuccess, nil
	})
	if err != nil {
		return fmt.Errorf("timed out waiting for cluster to start: %v", lastErr)
	}
	return nil
}
