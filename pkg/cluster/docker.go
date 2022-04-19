package cluster

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"

	"github.com/tilt-dev/ctlptl/internal/dctr"
)

type dockerClient interface {
	dctr.Client
	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (types.Info, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error
}
