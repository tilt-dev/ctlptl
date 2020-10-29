package cluster

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctlptlapi "github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestClusterGet(t *testing.T) {
	c := newFakeController(t)
	cluster, err := c.Get(context.Background(), "microk8s")
	assert.NoError(t, err)
	assert.Equal(t, cluster.Name, "microk8s")
	assert.Equal(t, cluster.Product, "microk8s")
}

func TestClusterList(t *testing.T) {
	c := newFakeController(t)
	clusters, err := c.List(context.Background(), ListOptions{})
	assert.NoError(t, err)
	require.Equal(t, 2, len(clusters))
	assert.Equal(t, "docker-desktop", clusters[0].Name)
	assert.Equal(t, "microk8s", clusters[1].Name)
}

func TestClusterListSelectorMatch(t *testing.T) {
	c := newFakeController(t)
	clusters, err := c.List(context.Background(), ListOptions{FieldSelector: "product=microk8s"})
	assert.NoError(t, err)
	require.Equal(t, 1, len(clusters))
	assert.Equal(t, "microk8s", clusters[0].Name)
}

func TestClusterListSelectorNoMatch(t *testing.T) {
	c := newFakeController(t)
	clusters, err := c.List(context.Background(), ListOptions{FieldSelector: "product=kind"})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(clusters))
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
	f.dmachine.os = "darwin"

	assert.Equal(t, false, f.d4m.started)
	kindAdmin := newFakeAdmin(f.config)
	f.controller.admins[ProductKIND] = kindAdmin

	result, err := f.controller.Apply(context.Background(), &ctlptlapi.Cluster{
		Product: string(ProductKIND),
	})
	assert.NoError(t, err)
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, "kind-kind", kindAdmin.created.Name)
	assert.Equal(t, "kind-kind", result.Name)
}

func TestClusterApplyDockerForMac(t *testing.T) {
	f := newFixture(t)
	f.dmachine.os = "darwin"

	assert.Equal(t, false, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
	f.controller.Apply(context.Background(), &ctlptlapi.Cluster{
		Product: string(ProductDockerDesktop),
		MinCPUs: 3,
	})
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 3, f.dockerClient.ncpu)
}

func TestClusterApplyDockerForMacCPUOnly(t *testing.T) {
	f := newFixture(t)
	f.dmachine.os = "darwin"

	err := f.d4m.start(context.Background())
	require.NoError(t, err)

	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
	f.controller.Apply(context.Background(), &ctlptlapi.Cluster{
		Product: string(ProductDockerDesktop),
		MinCPUs: 3,
	})
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 3, f.dockerClient.ncpu)
}

func TestClusterApplyDockerForMacStartClusterOnly(t *testing.T) {
	f := newFixture(t)
	f.dmachine.os = "darwin"

	assert.Equal(t, false, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
	f.controller.Apply(context.Background(), &ctlptlapi.Cluster{
		Product: string(ProductDockerDesktop),
		MinCPUs: 0,
	})
	assert.Equal(t, true, f.d4m.started)
	assert.Equal(t, 1, f.dockerClient.ncpu)
}

func TestClusterApplyDockerForMacNoRestart(t *testing.T) {
	f := newFixture(t)
	f.dmachine.os = "darwin"

	assert.Equal(t, 0, f.d4m.settingsWriteCount)
	f.controller.Apply(context.Background(), &ctlptlapi.Cluster{
		Product: string(ProductDockerDesktop),
	})
	assert.Equal(t, 1, f.d4m.settingsWriteCount)

	f.controller.Apply(context.Background(), &ctlptlapi.Cluster{
		Product: string(ProductDockerDesktop),
	})
	assert.Equal(t, 1, f.d4m.settingsWriteCount)
}

type fixture struct {
	t            *testing.T
	controller   *Controller
	dockerClient *fakeDockerClient
	dmachine     *dockerMachine
	d4m          *fakeD4MClient
	config       *clientcmdapi.Config
}

func newFixture(t *testing.T) *fixture {
	dockerClient := &fakeDockerClient{ncpu: 1}
	d4m := &fakeD4MClient{docker: dockerClient}
	dmachine := &dockerMachine{
		dockerClient: dockerClient,
		errOut:       os.Stderr,
		sleep:        func(d time.Duration) {},
		d4m:          d4m,
		os:           runtime.GOOS,
	}
	config := &clientcmdapi.Config{
		CurrentContext: "microk8s",
		Contexts: map[string]*api.Context{
			"microk8s": &api.Context{
				Cluster: "microk8s-cluster",
			},
			"docker-desktop": &api.Context{
				Cluster: "docker-desktop",
			},
		},
	}
	configLoader := configLoader(func() (clientcmdapi.Config, error) {
		return *config, nil
	})
	controller := &Controller{
		admins: make(map[Product]Admin),
		config: *config,
		clients: map[string]kubernetes.Interface{
			"microk8s": fake.NewSimpleClientset(),
		},
		dmachine:     dmachine,
		configLoader: configLoader,
	}
	return &fixture{
		t:            t,
		controller:   controller,
		dmachine:     dmachine,
		d4m:          d4m,
		dockerClient: dockerClient,
		config:       config,
	}
}

func newFakeController(t *testing.T) *Controller {
	return newFixture(t).controller
}

type fakeDockerClient struct {
	started bool
	ncpu    int
}

func (c *fakeDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	if !c.started {
		return types.Version{}, fmt.Errorf("not started")
	}

	return types.Version{}, nil
}

func (c *fakeDockerClient) Info(ctx context.Context) (types.Info, error) {
	if !c.started {
		return types.Info{}, fmt.Errorf("not started")
	}

	return types.Info{NCPU: c.ncpu}, nil
}

type fakeD4MClient struct {
	lastSettings       map[string]interface{}
	docker             *fakeDockerClient
	started            bool
	settingsWriteCount int
}

func (c *fakeD4MClient) writeSettings(ctx context.Context, settings map[string]interface{}) error {
	c.lastSettings = settings
	c.docker.ncpu = settings["cpu"].(int)
	c.settingsWriteCount++
	return nil
}

func (c *fakeD4MClient) settings(ctx context.Context) (map[string]interface{}, error) {
	return c.lastSettings, nil
}

func (c *fakeD4MClient) ensureK8sEnabled(settings map[string]interface{}) (bool, error) {
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

func (c *fakeD4MClient) start(ctx context.Context) error {
	c.lastSettings = map[string]interface{}{
		"cpu": c.docker.ncpu,
	}
	c.started = true
	c.docker.started = true
	return nil
}

type fakeAdmin struct {
	created *ctlptlapi.Cluster
	deleted *ctlptlapi.Cluster
	config  *clientcmdapi.Config
}

func newFakeAdmin(config *clientcmdapi.Config) *fakeAdmin {
	return &fakeAdmin{config: config}
}

func (a *fakeAdmin) EnsureInstalled(ctx context.Context) error { return nil }

func (a *fakeAdmin) Create(ctx context.Context, config *ctlptlapi.Cluster) error {
	a.created = config.DeepCopy()
	a.config.Contexts[config.Name] = &clientcmdapi.Context{Cluster: config.Name}
	return nil
}
func (a *fakeAdmin) Delete(ctx context.Context, config *ctlptlapi.Cluster) error {
	a.deleted = config.DeepCopy()
	delete(a.config.Contexts, config.Name)
	return nil
}
