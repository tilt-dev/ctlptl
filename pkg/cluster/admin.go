package cluster

import (
	"context"

	"github.com/tilt-dev/localregistry-go"

	"github.com/tilt-dev/ctlptl/internal/dctr"
	"github.com/tilt-dev/ctlptl/pkg/api"
)

// A cluster admin provides the basic start/stop functionality of a cluster,
// independent of the configuration of the machine it's running on.
type Admin interface {
	EnsureInstalled(ctx context.Context) error

	// Create a new cluster.
	//
	// Make a best effort attempt to delete any resources that might block creation
	// of the cluster.
	Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error

	// Infers the LocalRegistryHosting that this admin will try to configure.
	LocalRegistryHosting(ctx context.Context, desired *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error)

	Delete(ctx context.Context, config *api.Cluster) error
}

// An extension of cluster admin that indicates the cluster configuration can be
// modified for use from inside containers.
type AdminInContainer interface {
	ModifyConfigInContainer(ctx context.Context, cluster *api.Cluster, containerID string, dockerClient dctr.Client, configWriter configWriter) error
}

// Containerd made major changes to their config format for
// configuring registries. Each cluster has its own way
// of detecting this.

type containerdRegistryAPI int

const (
	containerdRegistryV1 containerdRegistryAPI = iota
	containerdRegistryV2
	containerdRegistryBroken
)
