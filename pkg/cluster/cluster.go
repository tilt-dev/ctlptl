package cluster

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/localregistry-go"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	// Client auth plugins! They will auto-init if we import them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var typeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "Cluster"}
var listTypeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "ClusterList"}
var groupResource = schema.GroupResource{"ctlptl.dev", "clusters"}

func TypeMeta() api.TypeMeta {
	return typeMeta
}
func ListTypeMeta() api.TypeMeta {
	return listTypeMeta
}

type configLoader func() (clientcmdapi.Config, error)

type Controller struct {
	iostreams    genericclioptions.IOStreams
	config       clientcmdapi.Config
	clients      map[string]kubernetes.Interface
	admins       map[Product]Admin
	dmachine     *dockerMachine
	configLoader configLoader
	mu           sync.Mutex
}

func DefaultController(iostreams genericclioptions.IOStreams) (*Controller, error) {
	configLoader := configLoader(func() (clientcmdapi.Config, error) {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

		overrides := &clientcmd.ConfigOverrides{}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
		return loader.RawConfig()
	})

	config, err := configLoader()
	if err != nil {
		return nil, err
	}

	return &Controller{
		iostreams:    iostreams,
		config:       config,
		clients:      make(map[string]kubernetes.Interface),
		admins:       make(map[Product]Admin),
		configLoader: configLoader,
	}, nil
}

func (c *Controller) machine(ctx context.Context, name string, product Product) (Machine, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch product {
	case ProductDockerDesktop, ProductKIND, ProductK3D:
		if c.dmachine == nil {
			machine, err := NewDockerMachine(ctx)
			if err != nil {
				return nil, err
			}
			c.dmachine = machine
		}
		return c.dmachine, nil

	case ProductMinikube:
		return minikubeMachine{name: name}, nil
	}

	return unknownMachine{product: product}, nil
}

// A cluster admin provides the basic start/stop functionality of a cluster,
// independent of the configuration of the machine it's running on.
func (c *Controller) admin(ctx context.Context, product Product) (Admin, error) {
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

	client, err = kubernetes.NewForConfig(restConfig)
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

func (c *Controller) populateCluster(ctx context.Context, cluster *api.Cluster) {
	name := cluster.Name
	client, err := c.client(cluster.Name)
	if err != nil {
		klog.V(4).Infof("WARNING: creating cluster %s client: %v", name, err)
		return
	}
	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := c.populateCreationTimestamp(ctx, cluster, client)
		if err != nil {
			klog.V(4).Infof("WARNING: reading cluster %s creation time: %v", name, err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := c.populateLocalRegistryHosting(ctx, cluster, client)
		if err != nil {
			klog.V(4).Infof("WARNING: reading cluster %s registry: %v", name, err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		err := c.populateMachineStatus(ctx, cluster)
		if err != nil {
			klog.V(4).Infof("WARNING: reading cluster %s machine: %v", name, err)
		}
	}()

	wg.Wait()
}

// Compare the desired cluster against the existing cluster, and reconcile
// the two to match.
func (c *Controller) Apply(ctx context.Context, desired *api.Cluster) (*api.Cluster, error) {
	if desired.Product == "" {
		return nil, fmt.Errorf("product field must be non-empty")
	}

	// Create a default name if one isn't in the YAML.
	// The default name is determiend by the underlying product.
	if desired.Name == "" {
		desired.Name = Product(desired.Product).DefaultClusterName()
	}

	// Fetch the admin driver for this product, for setting up the cluster on top of
	// the machine.
	admin, err := c.admin(ctx, Product(desired.Product))
	if err != nil {
		return nil, err
	}

	existingCluster, err := c.Get(ctx, desired.Name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if existingCluster == nil {
		existingCluster = &api.Cluster{}
	}

	existingStatus := existingCluster.Status

	// Configure the cluster to match what we want.
	needsCreate := existingStatus.CreationTimestamp.Time.IsZero() ||
		desired.Name != existingCluster.Name ||
		desired.Product != existingCluster.Product
	if needsCreate {
		err := admin.Create(ctx, desired)
		if err != nil {
			return nil, err
		}
	}

	err = c.reloadConfigs()
	if err != nil {
		return nil, err
	}

	return c.Get(ctx, desired.Name)
}

func (c *Controller) Delete(ctx context.Context, name string) error {
	_, ok := c.config.Contexts[name]
	if !ok {
		return errors.NewNotFound(groupResource, name)
	}

	fmt.Printf("Cluster Delete is currently a stub! You deleted: %s\n", name)
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

func (c *Controller) Get(ctx context.Context, name string) (*api.Cluster, error) {
	ct, ok := c.config.Contexts[name]
	if !ok {
		return nil, errors.NewNotFound(groupResource, name)
	}
	cluster := &api.Cluster{
		TypeMeta: typeMeta,
		Name:     name,
		Product:  productFromContext(ct, c.config.Clusters[ct.Cluster]).String(),
	}
	c.populateCluster(ctx, cluster)
	return cluster, nil
}

func (c *Controller) List(ctx context.Context, options ListOptions) (*api.ClusterList, error) {
	selector, err := fields.ParseSelector(options.FieldSelector)
	if err != nil {
		return nil, err
	}

	result := []api.Cluster{}
	names := make([]string, 0, len(c.config.Contexts))
	for name := range c.config.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		ct := c.config.Contexts[name]
		cluster := &api.Cluster{
			TypeMeta: typeMeta,
			Name:     name,
			Product:  productFromContext(ct, c.config.Clusters[ct.Cluster]).String(),
		}
		if !selector.Matches((*clusterFields)(cluster)) {
			continue
		}
		c.populateCluster(ctx, cluster)
		result = append(result, *cluster)
	}

	return &api.ClusterList{
		TypeMeta: listTypeMeta,
		Items:    result,
	}, nil
}
