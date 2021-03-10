package cluster

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/tilt-dev/localregistry-go"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

// k3dAdmin uses the k3d CLI to manipulate a k3d cluster,
// once the underlying machine has been setup.
type k3dAdmin struct {
	iostreams genericclioptions.IOStreams
}

func newK3dAdmin(iostreams genericclioptions.IOStreams) *k3dAdmin {
	return &k3dAdmin{
		iostreams: iostreams,
	}
}

func (a *k3dAdmin) EnsureInstalled(ctx context.Context) error {
	_, err := exec.LookPath("k3d")
	if err != nil {
		return fmt.Errorf("k3d not installed. Please install k3d with these instructions: https://k3d.io/#installation")
	}
	return nil
}

func (a *k3dAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	klog.V(3).Infof("Creating cluster with config:\n%+v\n---\n", desired)
	if registry != nil {
		klog.V(3).Infof("Initializing cluster with registry config:\n%+v\n---\n", registry)
	}

	clusterName := desired.Name
	if !strings.HasPrefix(clusterName, "k3d-") {
		return fmt.Errorf("all k3d clusters must have a name with the prefix k3d-*")
	}

	k3dName := strings.TrimPrefix(clusterName, "k3d-")

	args := []string{"cluster", "create", k3dName}
	if registry != nil {
		args = append(args, "--registry-use", registry.Name)
	}

	cmd := exec.CommandContext(ctx, "k3d", args...)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "creating k3d cluster")
	}

	return nil
}

// K3d manages the LocalRegistryHosting config itself :cheers:
func (a *k3dAdmin) LocalRegistryHosting(ctx context.Context, desired *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error) {
	return nil, nil
}

func (a *k3dAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	clusterName := config.Name
	if !strings.HasPrefix(clusterName, "k3d-") {
		return fmt.Errorf("all k3d clusters must have a name with the prefix k3d-*")
	}

	k3dName := strings.TrimPrefix(clusterName, "k3d-")
	cmd := exec.CommandContext(ctx, "k3d", "cluster", "delete", k3dName)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = a.iostreams.In
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "deleting k3d cluster")
	}
	return nil
}
