package cluster

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// A cluster admin provides the basic start/stop functionality of a cluster,
// independent of the configuration of the machine it's running on.
type Admin interface {
	EnsureInstalled(ctx context.Context) error
	Create(ctx context.Context, desired *api.Cluster) error
	Delete(ctx context.Context, config *api.Cluster) error
}

// dockerDesktopAdmin is currently a no-op admin.
// The dockerMachine driver automatically sets up Kubernetes
// when we're setting up the docker VM itself.
type dockerDesktopAdmin struct {
	os string
}

func newDockerDesktopAdmin() *dockerDesktopAdmin {
	return &dockerDesktopAdmin{os: runtime.GOOS}
}

func (a *dockerDesktopAdmin) EnsureInstalled(ctx context.Context) error { return nil }
func (a *dockerDesktopAdmin) Create(ctx context.Context, desired *api.Cluster) error {
	if a.os == "darwin" || a.os == "windows" {
		return nil
	}
	return fmt.Errorf("docker-desktop Kubernetes clusters are only available on macos and windows")
}

func (a *dockerDesktopAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	return fmt.Errorf("docker-desktop delete not implemented")
}

// kindAdmin uses the kind CLI to manipulate a kind cluster,
// once the underlying machine has been setup.
type kindAdmin struct {
	iostreams genericclioptions.IOStreams
}

func newKindAdmin(iostreams genericclioptions.IOStreams) *kindAdmin {
	return &kindAdmin{
		iostreams: iostreams,
	}
}

func (a *kindAdmin) EnsureInstalled(ctx context.Context) error {
	_, err := exec.LookPath("kind")
	if err != nil {
		return fmt.Errorf("kind not installed. Please install kind with these instructions: https://kind.sigs.k8s.io/")
	}
	return nil
}

func (a *kindAdmin) Create(ctx context.Context, desired *api.Cluster) error {
	clusterName := desired.Name
	if !strings.HasPrefix(clusterName, "kind-") {
		return fmt.Errorf("all kind clusters must have a name with the prefix kind-*")
	}

	kindName := strings.TrimPrefix(clusterName, "kind-")
	cmd := exec.CommandContext(ctx, "kind", "create", "cluster", "--name", kindName)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = a.iostreams.In
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}
	return nil
}

func (a *kindAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	clusterName := config.Name
	if !strings.HasPrefix(clusterName, "kind-") {
		return fmt.Errorf("all kind clusters must have a name with the prefix kind-*")
	}

	kindName := strings.TrimPrefix(clusterName, "kind-")
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", kindName)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = a.iostreams.In
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "deleting kind cluster")
	}
	return nil
}
