package registry

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/ctlptl/pkg/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var kindRegistry = types.Container{
	ID:      "a815c0ec15f1f7430bd402e3fffe65026dd692a1a99861a52b3e30ad6e253a08",
	Names:   []string{"/kind-registry"},
	Image:   "registry:2",
	ImageID: "sha256:2d4f4b5309b1e41b4f83ae59b44df6d673ef44433c734b14c1c103ebca82c116",
	Command: "/entrypoint.sh /etc/docker/registry/config.yml",
	Created: 1603483645,
	Ports: []types.Port{
		types.Port{IP: "0.0.0.0", PrivatePort: 5000, PublicPort: 5001, Type: "tcp"},
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

func TestListRegistries(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.docker.containers = []types.Container{kindRegistry}

	list, err := f.c.List(context.Background(), ListOptions{})
	require.NoError(t, err)

	require.Equal(t, 1, len(list.Items))
	assert.Equal(t, list.Items[0], api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483645, 0)},
			HostPort:          5001,
			ContainerPort:     5000,
			IPAddress:         "172.0.1.2",
			Networks:          []string{"bridge", "kind"},
		},
	})
}

func TestGetRegistry(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.docker.containers = []types.Container{kindRegistry}

	registry, err := f.c.Get(context.Background(), "kind-registry")
	require.NoError(t, err)
	assert.Equal(t, *registry, api.Registry{
		TypeMeta: typeMeta,
		Name:     "kind-registry",
		Status: api.RegistryStatus{
			CreationTimestamp: metav1.Time{Time: time.Unix(1603483645, 0)},
			HostPort:          5001,
			ContainerPort:     5000,
			IPAddress:         "172.0.1.2",
			Networks:          []string{"bridge", "kind"},
		},
	})
}

type fakeDocker struct {
	containers []types.Container
}

func (d *fakeDocker) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return d.containers, nil
}

type fixture struct {
	t      *testing.T
	c      *Controller
	docker *fakeDocker
}

func newFixture(t *testing.T) *fixture {
	d := &fakeDocker{}
	controller, err := NewController(d)
	require.NoError(t, err)
	return &fixture{
		t:      t,
		docker: d,
		c:      controller,
	}
}

func (fixture) TearDown() {}
