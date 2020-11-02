package cluster

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/localregistry-go"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// A cluster admin provides the basic start/stop functionality of a cluster,
// independent of the configuration of the machine it's running on.
type Admin interface {
	EnsureInstalled(ctx context.Context) error
	Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error
	LocalRegistryHosting(registry *api.Registry) *localregistry.LocalRegistryHostingV1
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
	return fmt.Errorf("docker-desktop delete not implemented")
}

const kindNetworkName = "kind"

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

func (a *kindAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	klog.V(3).Infof("Creating cluster with config:\n%+v\n---\n", desired)
	if registry != nil {
		klog.V(3).Infof("Initializing cluster with registry config:\n%+v\n---\n", registry)
	}

	clusterName := desired.Name
	if !strings.HasPrefix(clusterName, "kind-") {
		return fmt.Errorf("all kind clusters must have a name with the prefix kind-*")
	}

	kindName := strings.TrimPrefix(clusterName, "kind-")

	args := []string{"create", "cluster", "--name", kindName}

	// TODO(nick): Let the user pass in their own Kind configuration.
	in := strings.NewReader("")

	if registry != nil {
		containerdConfig := fmt.Sprintf(`
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:%d"]
    endpoint = ["http://%s:%d"]
`, registry.Status.HostPort, registry.Name, registry.Status.ContainerPort)
		in = strings.NewReader(containerdConfig)

		args = append(args, "--config", "-")
	}

	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = in
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}

	if !a.inKindNetwork(registry) {
		_, _ = fmt.Fprintf(a.iostreams.ErrOut, "   Connecting kind to registry %s\n", registry.Name)
		cmd := exec.CommandContext(ctx, "docker", "network", "connect", kindNetworkName, registry.Name)
		err := cmd.Run()
		if err != nil {
			return errors.Wrap(err, "connecting registry")
		}
	}

	return nil
}

func (a *kindAdmin) inKindNetwork(registry *api.Registry) bool {
	for _, n := range registry.Status.Networks {
		if n == kindNetworkName {
			return true
		}
	}
	return false
}

func (a *kindAdmin) LocalRegistryHosting(registry *api.Registry) *localregistry.LocalRegistryHostingV1 {
	return &localregistry.LocalRegistryHostingV1{
		Host:                   fmt.Sprintf("localhost:%d", registry.Status.HostPort),
		HostFromClusterNetwork: fmt.Sprintf("%s:%d", registry.Name, registry.Status.ContainerPort),
		Help:                   "https://github.com/tilt-dev/ctlptl",
	}
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
