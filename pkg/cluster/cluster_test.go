package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/localregistry-go"

	"github.com/tilt-dev/ctlptl/internal/dctr"
	"github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/registry"
)

func TestClusterGet(t *testing.T) {
	c := newFakeController(t)
	cluster, err := c.Get(context.Background(), "microk8s")
	assert.NoError(t, err)
	assert.Equal(t, cluster.Name, "microk8s")
	assert.Equal(t, cluster.Product, "microk8s")
}

func TestClusterCurrent(t *testing.T) {
	c := newFakeController(t)
	cluster, err := c.Current(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, cluster.Name, "microk8s")
	assert.Equal(t, cluster.Product, "microk8s")
}

func TestDeleteClusterContext(t *testing.T) {
	f := newFixture(t)

	admin := f.newFakeAdmin("docker-desktop")

	_, exists := f.config.Contexts["docker-desktop"]
	assert.True(t, exists)
	err := f.controller.Delete(context.Background(), "docker-desktop")
	assert.NoError(t, err)
	assert.Equal(t, "docker-desktop", admin.deleted.Name)

	_, exists = f.config.Contexts["docker-desktop"]
	assert.False(t, exists)
}

func TestClusterList(t *testing.T) {
	c := newFakeController(t)
	clusters, err := c.List(context.Background(), ListOptions{})
	assert.NoError(t, err)
	require.Equal(t, 2, len(clusters.Items))
	assert.Equal(t, "docker-desktop", clusters.Items[0].Name)
	assert.Equal(t, "microk8s", clusters.Items[1].Name)
}

func TestClusterStatusError(t *testing.T) {
	c := newFakeController(t)
	clusters, err := c.List(context.Background(), ListOptions{})
	assert.NoError(t, err)
	require.Equal(t, 2, len(clusters.Items))
	assert.Equal(t, "reading status: not started", clusters.Items[0].Status.Error)
	assert.Equal(t, "", clusters.Items[1].Status.Error)
}

// Make sure that an empty config doesn't confuse ctlptl.
func TestClusterListEmptyConfig(t *testing.T) {
	c := newFakeController(t)
	c.config.Contexts["kind"] = &clientcmdapi.Context{}

	clusters, err := c.List(context.Background(), ListOptions{})
	assert.NoError(t, err)
	require.Equal(t, 2, len(clusters.Items))
	assert.Equal(t, "docker-desktop", clusters.Items[0].Name)
	assert.Equal(t, "microk8s", clusters.Items[1].Name)
}

func TestClusterListSelectorMatch(t *testing.T) {
	c := newFakeController(t)
	clusters, err := c.List(context.Background(), ListOptions{FieldSelector: "product=microk8s"})
	assert.NoError(t, err)
	require.Equal(t, 1, len(clusters.Items))
	assert.Equal(t, "microk8s", clusters.Items[0].Name)
}

func TestClusterListSelectorNoMatch(t *testing.T) {
	c := newFakeController(t)
	clusters, err := c.List(context.Background(), ListOptions{FieldSelector: "product=kind"})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(clusters.Items))
}

func TestClusterGetMissing(t *testing.T) {
	c := newFakeController(t)
	_, err := c.Get(context.Background(), "dunkees")
	if assert.Error(t, err) {
		assert.True(t, errors.IsNotFound(err))
	}
}

func TestClusterApplyKIND(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	assert.Equal(t, false, f.d4m.started)
	kindAdmin := f.newFakeAdmin(clusterid.ProductKIND)

	result, err := f.controller.Apply(context.Background(), &api.Cluster{
		Product: string(clusterid.ProductKIND),
	})
	require.NoError(t, err)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, "kind-kind", kindAdmin.created.Name)
	assert.Equal(t, "kind-kind", result.Name)
}

// Make sure an empty context doesn't confuse ctlptl.
func TestClusterApplyKINDEmptyConfig(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	f.config.Contexts["kind"] = &clientcmdapi.Context{}

	assert.Equal(t, false, f.d4m.started)
	kindAdmin := f.newFakeAdmin(clusterid.ProductKIND)

	result, err := f.controller.Apply(context.Background(), &api.Cluster{
		Product: string(clusterid.ProductKIND),
	})
	require.NoError(t, err)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, "kind-kind", kindAdmin.created.Name)
	assert.Equal(t, "kind-kind", result.Name)
}

func TestClusterApplyFailsToStart(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	out := bytes.NewBuffer(nil)
	f.controller.iostreams.ErrOut = out

	assert.Equal(t, false, f.d4m.started)
	_ = f.newFakeAdmin(clusterid.ProductKIND)

	// Pretend that the kube-public namespace is never created.
	err := f.fakeK8s.CoreV1().Namespaces().Delete(
		context.Background(), "kube-public", metav1.DeleteOptions{})
	require.NoError(t, err)

	_, err = f.controller.Apply(context.Background(), &api.Cluster{
		Product: string(clusterid.ProductKIND),
	})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "timed out waiting for cluster to start")
		assert.Contains(t, out.String(), "Waiting 0s for Kubernetes cluster \"kind-kind\" to start")
	}
}

func TestClusterApplyKINDWithCluster(t *testing.T) {
	f := newFixture(t)
	f.setOS("linux")

	f.dockerClient.started = true

	kindAdmin := f.newFakeAdmin(clusterid.ProductKIND)

	result, err := f.controller.Apply(context.Background(), &api.Cluster{
		Product:  string(clusterid.ProductKIND),
		Registry: "kind-registry",
	})
	assert.NoError(t, err)
	assert.Equal(t, "kind-kind", result.Name)
	assert.Equal(t, "kind-registry", result.Registry)
	assert.Equal(t, "kind-registry", kindAdmin.createdRegistry.Name)
	assert.Equal(t, 5000, kindAdmin.createdRegistry.Status.ContainerPort)
	assert.Equal(t, "kind-registry", f.registryCtl.lastApply.Name)
}

func TestClusterApplyDockerDesktop(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	assert.Equal(t, false, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
	f.apply(clusterid.ProductDockerDesktop, 3)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 3, f.dockerClient.ncpu)
}

func TestClusterApplyDockerDesktopLinux(t *testing.T) {
	f := newFixture(t)
	f.setOS("linux")

	assert.Equal(t, false, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
	f.apply(clusterid.ProductDockerDesktop, 3)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 3, f.dockerClient.ncpu)
}

func TestClusterApplyDockerDesktopLinuxEngine(t *testing.T) {
	f := newFixture(t)
	f.setOS("linux")
	f.dockerClient.host = "unix:///var/run/docker.sock"

	cluster := &api.Cluster{
		Product: string(clusterid.ProductDockerDesktop),
	}
	_, err := f.controller.Apply(context.Background(), cluster)
	require.Error(f.t, err)
	require.Contains(f.t, err.Error(),
		"Not connected to Docker Engine. Host: \"unix:///var/run/docker.sock\". Error: not started")
}

func TestClusterApplyDockerDesktopCPUOnly(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	err := f.d4m.Open(context.Background())
	require.NoError(t, err)

	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
	f.apply(clusterid.ProductDockerDesktop, 3)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 3, f.dockerClient.ncpu)
}

func TestClusterApplyDockerDesktopStartClusterOnly(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	assert.Equal(t, false, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
	f.apply(clusterid.ProductDockerDesktop, 0)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
}

func TestClusterApplyDockerDesktopNoRestart(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	assert.Equal(t, 0, f.d4m.settingsWriteCount)

	// Pretend the cluster isn't running.
	err := f.fakeK8s.Tracker().Delete(schema.GroupVersionResource{Group: "", Version: "v1", Resource: "nodes"}, "", "node-1")
	assert.NoError(t, err)
	f.apply(clusterid.ProductDockerDesktop, 0)
	assert.Equal(t, 1, f.d4m.settingsWriteCount)
	assert.Equal(t, 1, f.d4m.resetCount)
	f.apply(clusterid.ProductDockerDesktop, 0)
	assert.Equal(t, 1, f.d4m.settingsWriteCount)
}

func TestClusterApplyMinikubeVersion(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	assert.Equal(t, false, f.d4m.started)
	minikubeAdmin := f.newFakeAdmin(clusterid.ProductMinikube)

	result, err := f.controller.Apply(context.Background(), &api.Cluster{
		Product:           string(clusterid.ProductMinikube),
		KubernetesVersion: "v1.14.0",
	})
	assert.NoError(t, err)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, "minikube", minikubeAdmin.created.Name)
	assert.Equal(t, "minikube", result.Name)
	assert.Equal(t, "minikube", f.config.CurrentContext)

	minikubeAdmin.created = nil

	_, err = f.controller.Apply(context.Background(), &api.Cluster{
		Product:           string(clusterid.ProductMinikube),
		KubernetesVersion: "v1.14.0",
	})
	assert.NoError(t, err)

	// Make sure we don't recreate the cluster.
	assert.Nil(t, minikubeAdmin.created)

	// Now, change the version and make sure we re-create the cluster.
	out := bytes.NewBuffer(nil)
	f.controller.iostreams.ErrOut = out

	_, err = f.controller.Apply(context.Background(), &api.Cluster{
		Product:           string(clusterid.ProductMinikube),
		KubernetesVersion: "v1.15.0",
	})
	assert.NoError(t, err)

	assert.Equal(t, "minikube", minikubeAdmin.created.Name)
	assert.Contains(t, out.String(),
		"Deleting cluster minikube because desired Kubernetes version (v1.15.0) "+
			"does not match current (v1.14.0)")
}

func TestFillDefaultsKindConfig(t *testing.T) {
	c := &api.Cluster{
		Product: "kind",
		KindV1Alpha4Cluster: &v1alpha4.Cluster{
			Name: "my-cluster",
		},
	}
	FillDefaults(c)
	assert.Equal(t, "kind-my-cluster", c.Name)

	c.KindV1Alpha4Cluster.Name = "your-cluster"
	FillDefaults(c)
	assert.Equal(t, "my-cluster", c.KindV1Alpha4Cluster.Name)
}

func TestClusterApplyKindConfig(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	assert.Equal(t, false, f.d4m.started)
	kindAdmin := f.newFakeAdmin(clusterid.ProductKIND)

	cluster := &api.Cluster{
		Product: string(clusterid.ProductKIND),
		KindV1Alpha4Cluster: &v1alpha4.Cluster{
			Nodes: []v1alpha4.Node{
				v1alpha4.Node{Role: "control-plane"},
			},
		},
	}
	_, err := f.controller.Apply(context.Background(), cluster)
	assert.NoError(t, err)
	assert.Equal(t, "kind-kind", kindAdmin.created.Name)
	kindAdmin.created = nil

	// Assert that re-applying the same config doesn't create a new cluster.
	_, err = f.controller.Apply(context.Background(), cluster)
	assert.NoError(t, err)
	assert.Nil(t, kindAdmin.created)
	assert.Nil(t, kindAdmin.deleted)

	// Assert that applying a different config deletes and re-creates.
	cluster2 := &api.Cluster{
		Product: string(clusterid.ProductKIND),
		KindV1Alpha4Cluster: &v1alpha4.Cluster{
			Nodes: []v1alpha4.Node{
				v1alpha4.Node{Role: "control-plane"},
				v1alpha4.Node{Role: "worker"},
			},
		},
	}

	f.errOut.Truncate(0)
	_, err = f.controller.Apply(context.Background(), cluster2)
	assert.NoError(t, err)
	assert.Equal(t, "kind-kind", kindAdmin.created.Name)
	assert.Equal(t, "kind-kind", kindAdmin.deleted.Name)
	assert.Contains(t, f.errOut.String(), "desired Kind config does not match current")
}

func TestClusterApplyMinikubeConfig(t *testing.T) {
	f := newFixture(t)
	f.setOS("darwin")

	assert.Equal(t, false, f.d4m.started)
	minikubeAdmin := f.newFakeAdmin(clusterid.ProductMinikube)

	cluster := &api.Cluster{
		Product: string(clusterid.ProductMinikube),
		Minikube: &api.MinikubeCluster{
			ContainerRuntime: "docker",
		},
	}
	_, err := f.controller.Apply(context.Background(), cluster)
	assert.NoError(t, err)
	assert.Equal(t, "minikube", minikubeAdmin.created.Name)
	minikubeAdmin.created = nil

	// Assert that re-applying the same config doesn't create a new cluster.
	_, err = f.controller.Apply(context.Background(), cluster)
	assert.NoError(t, err)
	assert.Nil(t, minikubeAdmin.created)
	assert.Nil(t, minikubeAdmin.deleted)

	// Assert that applying a different config deletes and re-creates.
	cluster2 := &api.Cluster{
		Product: string(clusterid.ProductMinikube),
		Minikube: &api.MinikubeCluster{
			ContainerRuntime: "containerd",
		},
	}

	f.errOut.Truncate(0)
	_, err = f.controller.Apply(context.Background(), cluster2)
	assert.NoError(t, err)
	assert.Equal(t, "minikube", minikubeAdmin.created.Name)
	assert.Equal(t, "minikube", minikubeAdmin.deleted.Name)
	assert.Contains(t, f.errOut.String(), "desired Minikube config does not match current")
}

func TestClusterFixKubeConfigInContainer(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	t.Run("when not in a container", func(t *testing.T) {
		cluster := &api.Cluster{Product: string(clusterid.ProductKIND)}
		assert.NoError(t, f.controller.maybeFixKubeConfigInsideContainer(ctx, cluster))
		assert.Empty(t, f.dockerClient.networks)
		assert.Empty(t, f.configWriter.opts)
	})

	t.Run("with a cluster that doesn't support AdminInContainer", func(t *testing.T) {
		f.dockerClient.containerID = "012345abcdef"
		cluster := &api.Cluster{Product: string(clusterid.ProductK3D)}
		assert.NoError(t, f.controller.maybeFixKubeConfigInsideContainer(ctx, cluster))
		assert.Empty(t, f.dockerClient.networks)
		assert.Empty(t, f.configWriter.opts)
	})

	t.Run("when in a container and using a KIND cluster", func(t *testing.T) {
		f.dockerClient.containerID = "012345abcdef"
		cluster := &api.Cluster{
			Name:    "kind-test",
			Product: string(clusterid.ProductKIND),
		}
		assert.NoError(t, f.controller.maybeFixKubeConfigInsideContainer(ctx, cluster))
		assert.Contains(t, f.dockerClient.networks, kindNetworkName())
		assert.Equal(t, "https://test-control-plane:6443", f.configWriter.opts["clusters.kind-test.server"])
	})
}

type fixture struct {
	t            *testing.T
	errOut       *bytes.Buffer
	controller   *Controller
	dockerClient *fakeDockerClient
	dmachine     *dockerMachine
	d4m          *fakeD4MClient
	config       *clientcmdapi.Config
	configWriter fakeConfigWriter
	registryCtl  *fakeRegistryController
	fakeK8s      *fake.Clientset
}

func newFixture(t *testing.T) *fixture {
	_ = os.Setenv("DOCKER_HOST", "")

	osName := "darwin" // default to macos
	dockerClient := &fakeDockerClient{host: "unix:///home/nick/.docker/desktop/docker.sock", ncpu: 1}
	d4m := &fakeD4MClient{docker: dockerClient}
	dmachine := &dockerMachine{
		dockerClient: dockerClient,
		iostreams:    genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr},
		sleep:        func(d time.Duration) {},
		d4m:          d4m,
		os:           osName,
	}
	config := &clientcmdapi.Config{
		CurrentContext: "microk8s",
		Contexts: map[string]*clientcmdapi.Context{
			"microk8s": &clientcmdapi.Context{
				Cluster: "microk8s-cluster",
			},
			"docker-desktop": &clientcmdapi.Context{
				Cluster: "docker-desktop",
			},
		},
		Clusters: map[string]*clientcmdapi.Cluster{
			"microk8s-cluster": &clientcmdapi.Cluster{Server: "http://microk8s.localhost/"},
			"docker-desktop":   &clientcmdapi.Cluster{Server: "http://docker-desktop.localhost/"},
		},
	}
	configLoader := configLoader(func() (clientcmdapi.Config, error) {
		return *config, nil
	})
	configWriter := fakeConfigWriter{config: config, opts: make(map[string]string)}
	iostreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    bytes.NewBuffer(nil),
		ErrOut: bytes.NewBuffer(nil),
	}
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "node-1",
			CreationTimestamp: metav1.Time{Time: time.Now()},
		},
	}
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "kube-public",
			CreationTimestamp: metav1.Time{Time: time.Now()},
		},
	}
	fakeK8s := fake.NewSimpleClientset(node, ns)
	clientLoader := clientLoader(func(restConfig *rest.Config) (kubernetes.Interface, error) {
		return fakeK8s, nil
	})

	registryCtl := &fakeRegistryController{}
	controller := &Controller{
		iostreams:                   iostreams,
		runner:                      exec.NewFakeCmdRunner(func(argv []string) string { return "" }),
		admins:                      make(map[clusterid.Product]Admin),
		config:                      *config,
		configWriter:                configWriter,
		dmachine:                    dmachine,
		configLoader:                configLoader,
		clientLoader:                clientLoader,
		clients:                     make(map[string]kubernetes.Interface),
		registryCtl:                 registryCtl,
		waitForKubeConfigTimeout:    time.Millisecond,
		waitForClusterCreateTimeout: time.Millisecond,
		os:                          osName,
		dockerCLI:                   &fakeCLI{client: dockerClient},
	}
	return &fixture{
		t:            t,
		errOut:       iostreams.ErrOut.(*bytes.Buffer),
		controller:   controller,
		dmachine:     dmachine,
		d4m:          d4m,
		dockerClient: dockerClient,
		config:       config,
		configWriter: configWriter,
		registryCtl:  registryCtl,
		fakeK8s:      fakeK8s,
	}
}

func (f *fixture) apply(product clusterid.Product, cpus int) {
	cluster := &api.Cluster{
		Product: string(product),
		MinCPUs: cpus,
	}
	_, err := f.controller.Apply(context.Background(), cluster)
	require.NoError(f.t, err)
}

func (f *fixture) setOS(os string) {
	f.controller.os = os
	f.dmachine.os = os
}

func (f *fixture) newFakeAdmin(p clusterid.Product) *fakeAdmin {
	admin := newFakeAdmin(f.config, f.fakeK8s)
	f.controller.admins[p] = admin
	return admin
}

func newFakeController(t *testing.T) *Controller {
	return newFixture(t).controller
}

type fakeCLI struct {
	client *fakeDockerClient
}

func (c *fakeCLI) Client() dctr.Client {
	return c.client
}

func (c *fakeCLI) AuthInfo(ctx context.Context, ref reference.Reference, cmdName string) (string, error) {
	return "", nil
}

type fakeDockerClient struct {
	started     bool
	ncpu        int
	host        string
	networks    []string
	containerID string
}

func (c *fakeDockerClient) DaemonHost() string {
	return c.host
}

func (c *fakeDockerClient) ServerVersion(ctx context.Context, options client.ServerVersionOptions) (client.ServerVersionResult, error) {
	if !c.started {
		return client.ServerVersionResult{}, fmt.Errorf("not started")
	}

	return client.ServerVersionResult{}, nil
}

func (c *fakeDockerClient) Info(ctx context.Context, options client.InfoOptions) (client.SystemInfoResult, error) {
	if !c.started {
		return client.SystemInfoResult{}, fmt.Errorf("not started")
	}

	return client.SystemInfoResult{Info: system.Info{NCPU: c.ncpu}}, nil
}

func (c *fakeDockerClient) ContainerInspect(ctx context.Context, id string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return client.ContainerInspectResult{}, nil
}

func (d *fakeDockerClient) ContainerRemove(ctx context.Context, id string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	return client.ContainerRemoveResult{}, nil
}

func (d *fakeDockerClient) ImagePull(ctx context.Context, image string, options client.ImagePullOptions) (client.ImagePullResponse, error) {
	return nil, nil
}

func (d *fakeDockerClient) ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error) {
	return client.ContainerListResult{}, nil
}

func (d *fakeDockerClient) ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	return client.ContainerCreateResult{}, nil
}
func (d *fakeDockerClient) ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error) {
	return client.ContainerStartResult{}, nil
}

func (d *fakeDockerClient) NetworkConnect(ctx context.Context, networkID string, options client.NetworkConnectOptions) (client.NetworkConnectResult, error) {
	d.networks = append(d.networks, networkID)
	return client.NetworkConnectResult{}, nil
}

func (d *fakeDockerClient) NetworkDisconnect(ctx context.Context, networkID string, options client.NetworkDisconnectOptions) (client.NetworkDisconnectResult, error) {
	if len(d.networks) == 0 {
		return client.NetworkDisconnectResult{}, nil
	}
	networks := []string{}
	for _, n := range d.networks {
		if n != networkID {
			networks = append(networks, n)
		}
	}
	d.networks = networks
	return client.NetworkDisconnectResult{}, nil
}

func (d *fakeDockerClient) insideContainer(ctx context.Context) string {
	return d.containerID
}

type fakeD4MClient struct {
	lastSettings       map[string]interface{}
	docker             *fakeDockerClient
	started            bool
	settingsWriteCount int
	resetCount         int
}

func (c *fakeD4MClient) writeSettings(ctx context.Context, settings map[string]interface{}) error {
	c.lastSettings = settings
	c.docker.ncpu = settings["cpu"].(int)
	c.settingsWriteCount++
	return nil
}

func (c *fakeD4MClient) settings(ctx context.Context) (map[string]interface{}, error) {
	if c.lastSettings == nil {
		c.lastSettings = make(map[string]interface{})
	}
	return c.lastSettings, nil
}

func (c *fakeD4MClient) setK8sEnabled(settings map[string]interface{}, desired bool) (bool, error) {
	enabled, ok := settings["k8sEnabled"]
	if ok && enabled.(bool) == true {
		return false, nil
	}
	settings["k8sEnabled"] = true
	return true, nil
}

func (c *fakeD4MClient) ensureMinCPU(settings map[string]interface{}, desired int) (bool, error) {
	cpu, ok := settings["cpu"]
	if ok && cpu.(int) >= desired {
		return false, nil
	}
	settings["cpu"] = desired
	return true, nil

}

func (c *fakeD4MClient) ResetCluster(ctx context.Context) error {
	c.resetCount++
	return nil
}

func (c *fakeD4MClient) Open(ctx context.Context) error {
	c.lastSettings = map[string]interface{}{
		"cpu": c.docker.ncpu,
	}
	c.started = true
	c.docker.started = true
	return nil
}

type fakeAdmin struct {
	created         *api.Cluster
	createdRegistry *api.Registry
	deleted         *api.Cluster
	config          *clientcmdapi.Config
	fakeK8s         *fake.Clientset
}

func newFakeAdmin(config *clientcmdapi.Config, fakeK8s *fake.Clientset) *fakeAdmin {
	return &fakeAdmin{config: config, fakeK8s: fakeK8s}
}

func (a *fakeAdmin) EnsureInstalled(ctx context.Context) error { return nil }

func (a *fakeAdmin) Create(ctx context.Context, config *api.Cluster, registry *api.Registry) error {
	a.created = config.DeepCopy()
	a.createdRegistry = registry.DeepCopy()
	a.config.Contexts[config.Name] = &clientcmdapi.Context{Cluster: config.Name}
	a.config.Clusters[config.Name] = &clientcmdapi.Cluster{Server: fmt.Sprintf("http://%s.localhost/", config.Name)}

	kVersion := config.KubernetesVersion
	if kVersion == "" {
		kVersion = "v1.19.1"
	}
	a.fakeK8s.Discovery().(*discoveryfake.FakeDiscovery).FakedServerVersion = &version.Info{
		GitVersion: kVersion,
	}
	return nil
}

func (a *fakeAdmin) LocalRegistryHosting(ctx context.Context, cluster *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error) {
	return &localregistry.LocalRegistryHostingV1{
		Host: fmt.Sprintf("localhost:%d", registry.Status.HostPort),
		Help: "https://github.com/tilt-dev/ctlptl",
	}, nil
}

func (a *fakeAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	a.deleted = config.DeepCopy()
	delete(a.config.Contexts, config.Name)
	return nil
}

type fakeRegistryController struct {
	lastApply *api.Registry
}

func (c *fakeRegistryController) List(ctx context.Context, options registry.ListOptions) (*api.RegistryList, error) {
	list := &api.RegistryList{}
	if c.lastApply != nil {
		item := c.lastApply.DeepCopy()
		list.Items = append(list.Items, *item)
	}
	return list, nil
}

func (c *fakeRegistryController) Apply(ctx context.Context, r *api.Registry) (*api.Registry, error) {
	c.lastApply = r.DeepCopy()

	newR := r.DeepCopy()
	newR.Status = api.RegistryStatus{
		ContainerPort: 5000,
		ContainerID:   "fake-container-id",
		HostPort:      5000,
		IPAddress:     "172.0.0.2",
		Networks:      []string{"bridge"},
	}
	return newR, nil
}

type fakeConfigWriter struct {
	config *clientcmdapi.Config
	opts   map[string]string
}

func (w fakeConfigWriter) SetContext(name string) error {
	w.config.CurrentContext = name
	return nil
}

func (w fakeConfigWriter) DeleteContext(name string) error {
	if w.config.CurrentContext == name {
		w.config.CurrentContext = ""
	}
	delete(w.config.Contexts, name)
	return nil
}

func (w fakeConfigWriter) SetConfig(name, value string) error {
	w.opts[name] = value
	return nil
}
