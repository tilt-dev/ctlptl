package registry

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

func kindRegistry() types.Container {
	return types.Container{
		ID:      "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
		Names:   []string{"/kind-registry"},
		Image:   "registry:2",
		ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483645,
		Labels:  map[string]string{"dev.tilt.ctlptl.role": "registry"},
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
		ID:      "d62f2587ff7b03858f144d3cf83c789578a6d6403f8b82a459ab4e317917cd42",
		Names:   []string{"/kind-registry-loopback"},
		Image:   "registry:2",
		ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483646,
		Labels:  map[string]string{"dev.tilt.ctlptl.role": "registry"},
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

func kindRegistryCustomImage() types.Container {
	return types.Container{
		ID:      "c7f123e65474f951c3bc4232c888616c0f9b1052c7ae706a3b6d4701bea6e90d",
		Names:   []string{"/kind-registry-custom-image"},
		Image:   "fake.tilt.dev/my-registry-image:latest",
		ImageID: "sha256:0ac33e5f5afa79e084075e8698a22d574816eea8d7b7d480586835657c3e1c8b",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483647,
		Labels:  map[string]string{"dev.tilt.ctlptl.role": "registry"},
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

	regWithoutLabels := kindRegistryLoopback()
	regWithoutLabels.Labels = nil

	f.docker.containers = []types.Container{kindRegistry(), regWithoutLabels, kindRegistryCustomImage()}

	list, err := f.c.List(context.Background(), ListOptions{})
	require.NoError(t, err)

	// registry list response is sorted by container ID:
	// 	kind-registry:a815, kind-registry-custom-image:c7f1, kind-registry-loopback:d62f
	require.Len(t, list.Items, 3)
	assert.Equal(t, api.Registry{
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
			Labels:            map[string]string{"dev.tilt.ctlptl.role": "registry"},
			Image:             "registry:2",
		},
	}, list.Items[0])
	assert.Equal(t, api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry-custom-image",
		Port:     5001,
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483647, 0)},
			HostPort:          5001,
			ContainerPort:     5000,
			IPAddress:         "172.0.1.2",
			ListenAddress:     "127.0.0.1",
			Networks:          []string{"bridge", "kind"},
			ContainerID:       "c7f123e65474f951c3bc4232c888616c0f9b1052c7ae706a3b6d4701bea6e90d",
			State:             "running",
			Labels:            map[string]string{"dev.tilt.ctlptl.role": "registry"},
			Image:             "fake.tilt.dev/my-registry-image:latest",
		},
	}, list.Items[1])
	assert.Equal(t, api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry-loopback",
		Port:     5001,
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483646, 0)},
			HostPort:          5001,
			ContainerPort:     5000,
			IPAddress:         "172.0.1.2",
			ListenAddress:     "127.0.0.1",
			Networks:          []string{"bridge", "kind"},
			ContainerID:       "d62f2587ff7b03858f144d3cf83c789578a6d6403f8b82a459ab4e317917cd42",
			State:             "running",
			Image:             "registry:2",
		},
	}, list.Items[2])
}

func TestGetRegistry(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.docker.containers = []types.Container{kindRegistry()}

	registry, err := f.c.Get(context.Background(), "kind-registry")
	require.NoError(t, err)
	assert.Equal(t, &api.Registry{
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
			Labels:            map[string]string{"dev.tilt.ctlptl.role": "registry"},
			Image:             "registry:2",
		},
	}, registry)
}

func TestApplyDeadRegistry(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	deadRegistry := kindRegistry()
	deadRegistry.State = "dead"
	f.docker.containers = []types.Container{deadRegistry}

	f.docker.onCreate = func() {
		f.docker.containers = []types.Container{kindRegistry()}
	}

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

	// Make sure the previous registry is wiped out
	// because it doesn't have the labels we want.
	f.docker.containers = []types.Container{kindRegistry()}

	f.docker.onCreate = func() {
		f.docker.containers = []types.Container{kindRegistry()}
	}

	registry, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Labels:   map[string]string{"managed-by": "ctlptl"},
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}
	config := f.docker.lastCreateConfig
	if assert.NotNil(t, config) {
		assert.Equal(t, map[string]string{
			"managed-by":           "ctlptl",
			"dev.tilt.ctlptl.role": "registry",
		}, config.Labels)
		assert.Equal(t, "kind-registry", config.Hostname)
		assert.Equal(t, "docker.io/library/registry:2", config.Image)
		assert.Equal(t, []string{"REGISTRY_STORAGE_DELETE_ENABLED=true"}, config.Env)
	}
}

func TestPreservePort(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	existingRegistry := kindRegistry()
	existingRegistry.State = "dead"
	existingRegistry.Ports[0].PublicPort = 5010
	f.docker.containers = []types.Container{existingRegistry}

	// Running a command makes the registry come alive!
	f.docker.onCreate = func() {
		f.docker.containers = []types.Container{kindRegistry()}
	}

	registry, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}

	config := f.docker.lastCreateConfig
	if assert.NotNil(t, config) {
		assert.Equal(t, map[string]string{"dev.tilt.ctlptl.role": "registry"}, config.Labels)
		assert.Equal(t, "kind-registry", config.Hostname)
		assert.Equal(t, "docker.io/library/registry:2", config.Image)
	}
}

func TestCustomImage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	// Make sure the previous registry is wiped out
	// because it doesn't have the image we want.
	f.docker.containers = []types.Container{kindRegistry()}

	f.docker.onCreate = func() {
		f.docker.containers = []types.Container{kindRegistry()}
	}

	// ensure stable w/o image change
	_, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Image:    "registry:2",
	})
	if assert.NoError(t, err) {
		assert.Nil(t, f.docker.lastCreateConfig, "Registry should not have been re-created")
	}

	// change image, should be (re)created
	registry, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Image:    "fake.tilt.dev/different-registry-image:latest",
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}
	config := f.docker.lastCreateConfig
	if assert.NotNil(t, config) {
		assert.Equal(t, map[string]string{"dev.tilt.ctlptl.role": "registry"}, config.Labels)
		assert.Equal(t, "kind-registry", config.Hostname)
		assert.Equal(t, "fake.tilt.dev/different-registry-image:latest", config.Image)
	}
}

type fakeDocker struct {
	containers           []types.Container
	lastRemovedContainer string
	lastCreateConfig     *container.Config
	lastCreateHostConfig *container.HostConfig
	onCreate             func()
}

type objectNotFoundError struct {
	object string
	id     string
}

func (e objectNotFoundError) NotFound() {}

func (e objectNotFoundError) Error() string {
	return fmt.Sprintf("Error: No such %s: %s", e.object, e.id)
}

func (d *fakeDocker) DaemonHost() string {
	return ""
}

func (d *fakeDocker) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	for _, c := range d.containers {
		if c.ID == containerID {
			return types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					State: &types.ContainerState{
						Running: c.State == "running",
					},
				},
			}, nil
		}
	}

	return types.ContainerJSON{}, objectNotFoundError{"container", containerID}
}

func (d *fakeDocker) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	var result []types.Container
	for _, c := range d.containers {
		if options.Filters.Contains("ancestor") {
			img, err := reference.ParseNormalizedNamed(c.Image)
			if err != nil || !options.Filters.Match("ancestor", img.String()) {
				continue
			}
		}
		if options.Filters.Contains("label") && !options.Filters.MatchKVList("label", c.Labels) {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}

func (d *fakeDocker) ContainerRemove(ctx context.Context, id string, options types.ContainerRemoveOptions) error {
	d.lastRemovedContainer = id
	return nil
}

func (d *fakeDocker) ImagePull(ctx context.Context, image string,
	options types.ImagePullOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (d *fakeDocker) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig, platform *specs.Platform,
	containerName string) (container.ContainerCreateCreatedBody, error) {
	d.lastCreateConfig = config
	d.lastCreateHostConfig = hostConfig
	if d.onCreate != nil {
		d.onCreate()
	}
	return container.ContainerCreateCreatedBody{}, nil
}
func (d *fakeDocker) ContainerStart(ctx context.Context, containerID string,
	options types.ContainerStartOptions) error {
	return nil
}

type fixture struct {
	t      *testing.T
	c      *Controller
	docker *fakeDocker
}

func newFixture(t *testing.T) *fixture {
	_ = os.Setenv("DOCKER_HOST", "")

	d := &fakeDocker{}
	controller := NewController(
		genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}, d)
	return &fixture{
		t:      t,
		docker: d,
		c:      controller,
	}
}

func (fixture) TearDown() {}
