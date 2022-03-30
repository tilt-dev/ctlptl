package cluster

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/tilt-dev/ctlptl/pkg/docker"
)

type dockerClient interface {
	IsLocalHost() bool
	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (types.Info, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerRemove(ctx context.Context, id string, options types.ContainerRemoveOptions) error
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error
}

type dockerWrapper struct {
	*client.Client
	isLocalHost bool
}

func (w *dockerWrapper) IsLocalHost() bool { return w.isLocalHost }

func newDockerWrapperFromEnv(ctx context.Context) (*dockerWrapper, error) {
	opts, err := docker.ClientOpts()
	if err != nil {
		return nil, err
	}
	c, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}

	c.NegotiateAPIVersion(ctx)
	isLocalHost := docker.IsLocalHost(docker.GetHostEnv())
	return &dockerWrapper{
		Client:      c,
		isLocalHost: isLocalHost,
	}, nil
}
