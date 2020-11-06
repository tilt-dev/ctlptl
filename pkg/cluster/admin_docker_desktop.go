package cluster

import (
	"context"
	"fmt"
	"runtime"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/localregistry-go"
)

// The DockerDesktop manages the Kubernetes cluster for DockerDesktop.
// This is a bit different than the other admins, due to the overlap
//
type dockerDesktopAdmin struct {
	os string
}

func newDockerDesktopAdmin() *dockerDesktopAdmin {
	return &dockerDesktopAdmin{os: runtime.GOOS}
}

func (a *dockerDesktopAdmin) EnsureInstalled(ctx context.Context) error { return nil }
func (a *dockerDesktopAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	if registry != nil {
		return fmt.Errorf("ctlptl currently does not support connecting a registry to docker-desktop")
	}

	if a.os == "darwin" || a.os == "windows" {
		return nil
	}
	return fmt.Errorf("docker-desktop Kubernetes clusters are only available on macos and windows")
}

func (a *dockerDesktopAdmin) LocalRegistryHosting(registry *api.Registry) *localregistry.LocalRegistryHostingV1 {
	return nil
}

func (a *dockerDesktopAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return fmt.Errorf("docker-desktop delete not implemented on: %s", runtime.GOOS)
	}

	client, err := NewDockerDesktopClient()
	if err != nil {
		return err
	}

	err = client.resetK8s(ctx)
	if err != nil {
		return err
	}

	settings, err := client.settings(ctx)
	if err != nil {
		return err
	}

	changed, err := client.setK8sEnabled(settings, false)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	return client.writeSettings(ctx, settings)
}
