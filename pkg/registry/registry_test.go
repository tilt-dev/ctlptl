package registry

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func kindRegistry() types.Container {
	return types.Container{
		ID:      "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
		Names:   []string{"/kind-registry"},
		Image:   "registry:2",
		ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483645,
		Ports: []types.Port{
			types.Port{IP: "127.0.0.1", PrivatePort: 5000, PublicPort: 5001, Type: "tcp"},
		},
		SizeRw:     0,
		SizeRootFs: 0,
		State:      "running",
		Status:     "Up 2 hours",
		NetworkSettings: &types.SummaryNetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"bridge": &network.EndpointSettings{
					IPAddress: "172.0.1.2",
				},
				"kind": &network.EndpointSettings{
					IPAddress: "172.0.1.3",
				},
			},
		},
	}
}

func kindRegistryLoopback() types.Container {
	return types.Container{
		ID:      "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
		Names:   []string{"/kind-registry-loopback"},
		Image:   "registry:2",
		ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483645,
		Ports: []types.Port{
			types.Port{IP: "127.0.0.1", PrivatePort: 5000, PublicPort: 5001, Type: "tcp"},
		},
		SizeRw:     0,
		SizeRootFs: 0,
		State:      "running",
		Status:     "Up 2 hours",
		NetworkSettings: &types.SummaryNetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"bridge": &network.EndpointSettings{
					IPAddress: "172.0.1.2",
				},
				"kind": &network.EndpointSettings{
					IPAddress: "172.0.1.3",
				},
			},
		},
	}
}

func TestListRegistries(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.docker.containers = []types.Container{kindRegistry(), kindRegistryLoopback()}

	list, err := f.c.List(context.Background(), ListOptions{})
	require.NoError(t, err)

	require.Equal(t, 2, len(list.Items))
	assert.Equal(t, list.Items[0], api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Port:     5001,
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483645, 0)},
			HostPort:          5001,
			ContainerPort:     5000,
			IPAddress:         "172.0.1.2",
			ListenAddress:     "127.0.0.1",
			Networks:          []string{"bridge", "kind"},
			ContainerID:       "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
			State:             "running",
		},
	})
	assert.Equal(t, list.Items[1], api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry-loopback",
		Port:     5001,
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483645, 0)},
			HostPort:          5001,
			ContainerPort:     5000,
			IPAddress:         "172.0.1.2",
			ListenAddress:     "127.0.0.1",
			Networks:          []string{"bridge", "kind"},
			ContainerID:       "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
			State:             "running",
		},
	})
}

func TestGetRegistry(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.docker.containers = []types.Container{kindRegistry()}

	registry, err := f.c.Get(context.Background(), "kind-registry")
	require.NoError(t, err)
	assert.Equal(t, *registry, api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Port:     5001,
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483645, 0)},
			HostPort:          5001,
			ContainerPort:     5000,
			IPAddress:         "172.0.1.2",
			ListenAddress:     "127.0.0.1",
			Networks:          []string{"bridge", "kind"},
			ContainerID:       "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
			State:             "running",
		},
	})
}

func TestApplyDeadRegistry(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	deadRegistry := kindRegistry()
	deadRegistry.State = "dead"
	f.docker.containers = []types.Container{deadRegistry}

	// Running a command makes the registry come alive!
	f.c.runner = exec.NewFakeCmdRunner(func(argv []string) {
		assert.Equal(t, "docker", argv[0])
		assert.Equal(t, "run", argv[1])
		f.docker.containers = []types.Container{kindRegistry()}
	})

	registry, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Port:     5001,
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}
	assert.Equal(t, deadRegistry.ID, f.docker.lastRemovedContainer)
}

func TestApplyLabels(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	// Running a command makes the registry come alive!
	f.runner = exec.NewFakeCmdRunner(func(argv []string) {
		f.docker.containers = []types.Container{kindRegistry()}
	})
	f.c.runner = f.runner

	registry, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Port:     5001,
		Labels:   map[string]string{"managed-by": "ctlptl"},
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}
	assert.Equal(t, f.runner.LastArgs, []string{
		"docker", "run", "-d", "--restart=always",
		"--name", "kind-registry",
		"-p", "127.0.0.1:5001:5000",
		"-l=managed-by=ctlptl",
		"docker.io/library/registry:2",
	})
}

func TestPreservePort(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	existingRegistry := kindRegistry()
	existingRegistry.State = "dead"
	existingRegistry.Ports[0].PublicPort = 5010
	f.docker.containers = []types.Container{existingRegistry}

	// Running a command makes the registry come alive!
	f.runner = exec.NewFakeCmdRunner(func(argv []string) {
		f.docker.containers = []types.Container{kindRegistry()}
	})
	f.c.runner = f.runner

	registry, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}
	assert.Equal(t, f.runner.LastArgs, []string{
		"docker", "run", "-d", "--restart=always",
		"--name", "kind-registry",
		"-p", "127.0.0.1:5010:5000",
		"docker.io/library/registry:2",
	})
}

type fakeDocker struct {
	containers           []types.Container
	lastRemovedContainer string
}

func (d *fakeDocker) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return types.ContainerJSON{}, nil
}

func (d *fakeDocker) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return d.containers, nil
}

func (d *fakeDocker) ContainerRemove(ctx context.Context, id string, options types.ContainerRemoveOptions) error {
	d.lastRemovedContainer = id
	return nil
}

type fixture struct {
	t      *testing.T
	c      *Controller
	docker *fakeDocker
	runner *exec.FakeCmdRunner
}

func newFixture(t *testing.T) *fixture {
	_ = os.Setenv("DOCKER_HOST", "")

	d := &fakeDocker{}
	controller, err := NewController(
		genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}, d)
	runner := exec.NewFakeCmdRunner(func(argv []string) {
		log.Println("No runner installed")
	})
	controller.runner = runner
	require.NoError(t, err)
	return &fixture{
		t:      t,
		docker: d,
		c:      controller,
		runner: runner,
	}
}

func (fixture) TearDown() {}
