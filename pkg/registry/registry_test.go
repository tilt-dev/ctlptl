package registry

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/registry"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/internal/dctr"
	"github.com/tilt-dev/ctlptl/pkg/api"
)

func kindRegistry() container.Summary {
	return container.Summary{
		ID:      "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
		Names:   []string{"/kind-registry"},
		Image:   DefaultRegistryImageRef,
		ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483645,
		Labels:  map[string]string{"dev.tilt.ctlptl.role": "registry"},
		Ports: []container.Port{
			container.Port{IP: "127.0.0.1", PrivatePort: 5000, PublicPort: 5001, Type: "tcp"},
		},
		SizeRw:     0,
		SizeRootFs: 0,
		State:      "running",
		Status:     "Up 2 hours",
		NetworkSettings: &container.NetworkSettingsSummary{
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

func kindRegistryLoopback() container.Summary {
	return container.Summary{
		ID:      "d62f2587ff7b03858f144d3cf83c789578a6d6403f8b82a459ab4e317917cd42",
		Names:   []string{"/kind-registry-loopback"},
		Image:   DefaultRegistryImageRef,
		ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483646,
		Labels:  map[string]string{"dev.tilt.ctlptl.role": "registry"},
		Ports: []container.Port{
			container.Port{IP: "127.0.0.1", PrivatePort: 5000, PublicPort: 5001, Type: "tcp"},
		},
		SizeRw:     0,
		SizeRootFs: 0,
		State:      "running",
		Status:     "Up 2 hours",
		NetworkSettings: &container.NetworkSettingsSummary{
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

func kindRegistryCustomImage() container.Summary {
	return container.Summary{
		ID:      "c7f123e65474f951c3bc4232c888616c0f9b1052c7ae706a3b6d4701bea6e90d",
		Names:   []string{"/kind-registry-custom-image"},
		Image:   "fake.tilt.dev/my-registry-image:latest",
		ImageID: "sha256:0ac33e5f5afa79e084075e8698a22d574816eea8d7b7d480586835657c3e1c8b",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483647,
		Labels:  map[string]string{"dev.tilt.ctlptl.role": "registry"},
		Ports: []container.Port{
			container.Port{IP: "127.0.0.1", PrivatePort: 5000, PublicPort: 5001, Type: "tcp"},
		},
		SizeRw:     0,
		SizeRootFs: 0,
		State:      "running",
		Status:     "Up 2 hours",
		NetworkSettings: &container.NetworkSettingsSummary{
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

func registryBadPorts() container.Summary {
	return container.Summary{
		ID:      "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
		Names:   []string{"/kind-registry"},
		Image:   DefaultRegistryImageRef,
		ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
		Command: "/entrypoint.sh /etc/docker/registry/config.yml",
		Created: 1603483645,
		Labels:  map[string]string{"dev.tilt.ctlptl.role": "registry"},
		Ports: []container.Port{
			container.Port{IP: "127.0.0.1", PrivatePort: 5001, PublicPort: 5002, Type: "tcp"},
		},
		SizeRw:     0,
		SizeRootFs: 0,
		State:      "running",
		Status:     "Up 2 hours",
		NetworkSettings: &container.NetworkSettingsSummary{
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

	f.docker.containers = []container.Summary{kindRegistry(), regWithoutLabels, kindRegistryCustomImage()}

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
			Image:             DefaultRegistryImageRef,
			Env:               []string{"REGISTRY_STORAGE_DELETE_ENABLED=true", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
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
			Env:               []string{"REGISTRY_STORAGE_DELETE_ENABLED=true", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
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
			Image:             DefaultRegistryImageRef,
			Env:               []string{"REGISTRY_STORAGE_DELETE_ENABLED=true", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		},
	}, list.Items[2])
}

func TestListRegistries_badPorts(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	regWithoutLabels := kindRegistryLoopback()
	regWithoutLabels.Labels = nil

	f.docker.containers = []container.Summary{registryBadPorts()}

	list, err := f.c.List(context.Background(), ListOptions{})
	require.NoError(t, err)

	require.Len(t, list.Items, 1)
	assert.Equal(t, api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483645, 0)},
			IPAddress:         "172.0.1.2",
			Networks:          []string{"bridge", "kind"},
			ContainerID:       "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
			State:             "running",
			Labels:            map[string]string{"dev.tilt.ctlptl.role": "registry"},
			Image:             DefaultRegistryImageRef,
			Env:               []string{"REGISTRY_STORAGE_DELETE_ENABLED=true", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			Warnings: []string{
				"Unexpected registry ports: [{IP:127.0.0.1 PrivatePort:5001 PublicPort:5002 Type:tcp}]",
			},
		},
	}, list.Items[0])
}

func TestGetRegistry(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.docker.containers = []container.Summary{kindRegistry()}

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
			Image:             DefaultRegistryImageRef,
			Env:               []string{"REGISTRY_STORAGE_DELETE_ENABLED=true", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		},
	}, registry)
}

func TestApplyDeadRegistry(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	deadRegistry := kindRegistry()
	deadRegistry.State = "dead"
	f.docker.containers = []container.Summary{deadRegistry}

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
	f.docker.containers = []container.Summary{kindRegistry()}

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
		assert.Equal(t, DefaultRegistryImageRef, config.Image)
		assert.Equal(t, []string{"REGISTRY_STORAGE_DELETE_ENABLED=true"}, config.Env)
	}
}

func TestPreservePort(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	existingRegistry := kindRegistry()
	existingRegistry.State = "dead"
	existingRegistry.Ports[0].PublicPort = 5010
	f.docker.containers = []container.Summary{existingRegistry}

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
		assert.Equal(t, DefaultRegistryImageRef, config.Image)
	}
}

func TestCustomImage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	// Make sure the previous registry is wiped out
	// because it doesn't have the image we want.
	f.docker.containers = []container.Summary{kindRegistry()}

	// ensure stable w/o image change
	_, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Image:    DefaultRegistryImageRef,
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

	// Apply a config with new labels,
	// ensure image is not changed.
	registry, err = f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Labels:   map[string]string{"extra-label": "ctlptl"},
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}
	config = f.docker.lastCreateConfig
	if assert.NotNil(t, config) {
		assert.Equal(t, map[string]string{
			"dev.tilt.ctlptl.role": "registry",
			"extra-label":          "ctlptl",
		}, config.Labels)
		assert.Equal(t, "kind-registry", config.Hostname)
		assert.Equal(t, "fake.tilt.dev/different-registry-image:latest", config.Image)
	}
}

func TestCustomEnv(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	// Make sure the previous registry is wiped out
	// because it doesn't have the image we want.
	f.docker.containers = []container.Summary{kindRegistry()}

	// ensure stable w/o image change
	_, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Image:    DefaultRegistryImageRef,
	})
	if assert.NoError(t, err) {
		assert.Nil(t, f.docker.lastCreateConfig, "Registry should not have been re-created")
	}

	// change env, should be (re)created
	registry, err := f.c.Apply(context.Background(), &api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Image:    DefaultRegistryImageRef,
		Env:      []string{"REGISTRY_STORAGE_DELETE_ENABLED=false"},
	})
	if assert.NoError(t, err) {
		assert.Equal(t, "running", registry.Status.State)
	}
	config := f.docker.lastCreateConfig
	if assert.NotNil(t, config) {
		assert.Equal(t, map[string]string{"dev.tilt.ctlptl.role": "registry"}, config.Labels)
		assert.Equal(t, "kind-registry", config.Hostname)
		assert.Equal(t, DefaultRegistryImageRef, config.Image)
		assert.Equal(t, []string{"REGISTRY_STORAGE_DELETE_ENABLED=false"}, config.Env)
	}
}

type fakeCLI struct {
	client *fakeDocker
}

func (c *fakeCLI) Client() dctr.Client {
	return c.client
}

func (c *fakeCLI) AuthInfo(ctx context.Context, repoInfo *registry.RepositoryInfo, cmdName string) (string, registrytypes.RequestAuthConfig, error) {
	return "", nil, nil
}

type fakeDocker struct {
	containers           []container.Summary
	lastRemovedContainer string
	lastCreateConfig     *container.Config
	lastCreateHostConfig *container.HostConfig
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

func (d *fakeDocker) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	for _, c := range d.containers {
		if c.ID == containerID {
			return container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					State: &container.State{
						Running: c.State == "running",
					},
				},
				Config: &container.Config{
					Hostname:     "test",
					Domainname:   "",
					User:         "",
					AttachStdin:  false,
					AttachStdout: false,
					AttachStderr: false,
					// ExposedPorts:nat.PortSet{"5000/tcp":struct {}{}},
					Tty:             false,
					OpenStdin:       false,
					StdinOnce:       false,
					Env:             []string{"REGISTRY_STORAGE_DELETE_ENABLED=true", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
					Cmd:             []string{"/etc/docker/registry/config.yml"},
					Healthcheck:     (*container.HealthConfig)(nil),
					ArgsEscaped:     false,
					Image:           DefaultRegistryImageRef,
					Volumes:         map[string]struct{}{"/var/lib/registry": struct{}{}},
					WorkingDir:      "",
					Entrypoint:      []string{"/entrypoint.sh"},
					NetworkDisabled: false,
					MacAddress:      "",
					OnBuild:         []string(nil),
					Labels:          map[string]string{"dev.tilt.ctlptl.role": "registry"},
					StopSignal:      "",
					StopTimeout:     (*int)(nil),
					Shell:           []string(nil),
				},
			}, nil
		}
	}

	return container.InspectResponse{}, objectNotFoundError{"container", containerID}
}

func (d *fakeDocker) ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	var result []container.Summary
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

func (d *fakeDocker) ContainerRemove(ctx context.Context, id string, options container.RemoveOptions) error {
	d.lastRemovedContainer = id
	return nil
}

func (d *fakeDocker) ImagePull(ctx context.Context, image string,
	options image.PullOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (d *fakeDocker) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig, platform *specs.Platform,
	containerName string) (container.CreateResponse, error) {
	d.lastCreateConfig = config
	d.lastCreateHostConfig = hostConfig

	c := kindRegistry()
	c.Image = config.Image
	d.containers = []container.Summary{c}

	return container.CreateResponse{}, nil
}
func (d *fakeDocker) ContainerStart(ctx context.Context, containerID string,
	options container.StartOptions) error {
	return nil
}
func (d *fakeDocker) ServerVersion(ctx context.Context) (types.Version, error) {
	return types.Version{}, nil
}
func (d *fakeDocker) Info(ctx context.Context) (system.Info, error) {
	return system.Info{}, nil
}
func (d *fakeDocker) NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error {
	return nil
}
func (d *fakeDocker) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
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
		genericclioptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		&fakeCLI{client: d})
	return &fixture{
		t:      t,
		docker: d,
		c:      controller,
	}
}

func (fixture) TearDown() {}
