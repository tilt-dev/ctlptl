package cluster

import (
	"context"

	"github.com/docker/docker/api/types"
)

type dockerClient interface {
	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (types.Info, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerRemove(ctx context.Context, id string, options types.ContainerRemoveOptions) error
}
