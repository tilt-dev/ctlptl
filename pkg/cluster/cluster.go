package cluster

import (
	"context"
	"fmt"
	"sync"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/localregistry-go"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog/v2"

	// Client auth plugins! They will auto-init if we import them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var typeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "Cluster"}
var groupResource = schema.GroupResource{"ctlptl.dev", "clusters"}

type Controller struct {
	config   clientcmdapi.Config
	clients  map[string]kubernetes.Interface
	dmachine *dockerMachine
	mu       sync.Mutex
}

func ControllerWithConfig(config clientcmdapi.Config) *Controller {
	return &Controller{
		config:  config,
		clients: make(map[string]kubernetes.Interface),
	}
}

func DefaultController() (*Controller, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)
	rawConfig, err := loader.RawConfig()
	if err != nil {
		return nil, err
	}
	return ControllerWithConfig(rawConfig), nil
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
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if existingCluster == nil {
		existingCluster = &api.Cluster{}
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

	return c.Get(ctx, desired.Name)
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

func (c *Controller) List(ctx context.Context, options ListOptions) ([]*api.Cluster, error) {
	selector, err := fields.ParseSelector(options.FieldSelector)
	if err != nil {
		return nil, err
	}

	result := []*api.Cluster{}
	for name, ct := range c.config.Contexts {
		cluster := &api.Cluster{
			TypeMeta: typeMeta,
			Name:     name,
			Product:  productFromContext(ct, c.config.Clusters[ct.Cluster]).String(),
		}
		if !selector.Matches((*clusterFields)(cluster)) {
			continue
		}
		c.populateCluster(ctx, cluster)
		result = append(result, cluster)
	}
	return result, nil
}
