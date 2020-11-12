package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/localregistry-go"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// minikubeAdmin uses the minikube CLI to manipulate a minikube cluster,
// once the underlying machine has been setup.
type minikubeAdmin struct {
	iostreams genericclioptions.IOStreams
}

func newMinikubeAdmin(iostreams genericclioptions.IOStreams) *minikubeAdmin {
	return &minikubeAdmin{
		iostreams: iostreams,
	}
}

func (a *minikubeAdmin) EnsureInstalled(ctx context.Context) error {
	_, err := exec.LookPath("minikube")
	if err != nil {
		return fmt.Errorf("minikube not installed. Please install minikube with these instructions: https://minikube.sigs.k8s.io/")
	}
	return nil
}

func (a *minikubeAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	klog.V(3).Infof("Creating cluster with config:\n%+v\n---\n", desired)
	if registry != nil {
		klog.V(3).Infof("Initializing cluster with registry config:\n%+v\n---\n", registry)
	}

	clusterName := desired.Name

	// TODO(nick): Let the user pass in their own Minikube configuration.
	args := []string{"start", "--driver=docker", "--container-runtime=containerd", "-p", clusterName}
	if desired.MinCPUs != 0 {
		args = append(args, fmt.Sprintf("--cpus=%d", desired.MinCPUs))
	}
	if desired.KubernetesVersion != "" {
		args = append(args, "--kubernetes-version", desired.KubernetesVersion)
	}

	in := strings.NewReader("")

	cmd := exec.CommandContext(ctx, "minikube", args...)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = in
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "creating minikube cluster")
	}

	if registry != nil {
		err = a.applyContainerdPatch(ctx, desired, registry)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *minikubeAdmin) applyContainerdPatch(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	configPath := "/etc/containerd/config.toml"

	nodeOutput := bytes.NewBuffer(nil)
	cmd := exec.CommandContext(ctx, "minikube", "-p", desired.Name, "node", "list")
	cmd.Stdout = nodeOutput
	cmd.Stderr = a.iostreams.ErrOut
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "configuring minikube registry")
	}

	nodes := []string{}
	nodeOutputSplit := strings.Split(nodeOutput.String(), "\n")
	for _, line := range nodeOutputSplit {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		node := strings.TrimSpace(fields[0])
		if node == "" {
			continue
		}
		nodes = append(nodes, node)
	}

	for _, node := range nodes {
		// this is the most annoying sed expression i've ever had to write
		// minikube does not give us great primitives for writing files on the host machine :\
		// so we have to hack around the shell escaping on its interactive shell
		cmd := exec.CommandContext(ctx, "minikube", "-p", desired.Name, "--node", node,
			"ssh", "sudo", "sed", `\-i`,
			fmt.Sprintf(
				`s,\\\[plugins.cri.registry.mirrors\\\],[plugins.cri.registry.mirrors]\\\n`+
					`\ \ \ \ \ \ \ \ [plugins.cri.registry.mirrors.\\\"localhost:%d\\\"]\\\n`+
					`\ \ \ \ \ \ \ \ \ \ endpoint\ =\ [\\\"http://%s:%d\\\"],`,
				registry.Status.HostPort, registry.Status.IPAddress, registry.Status.ContainerPort),
			configPath)
		cmd.Stderr = a.iostreams.ErrOut
		cmd.Stdout = a.iostreams.Out
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "configuring minikube registry")
		}

		cmd = exec.CommandContext(ctx, "minikube", "-p", desired.Name, "--node", node,
			"ssh", "sudo", "systemctl", "restart", "containerd")
		cmd.Stderr = a.iostreams.ErrOut
		cmd.Stdout = a.iostreams.Out
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "configuring minikube registry")
		}
	}
	return nil
}

func (a *minikubeAdmin) LocalRegistryHosting(registry *api.Registry) *localregistry.LocalRegistryHostingV1 {
	return &localregistry.LocalRegistryHostingV1{
		Host: fmt.Sprintf("localhost:%d", registry.Status.HostPort),
		Help: "https://github.com/tilt-dev/ctlptl",
	}
}

func (a *minikubeAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	cmd := exec.CommandContext(ctx, "minikube", "delete", "-p", config.Name)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = a.iostreams.In
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "deleting minikube cluster")
	}
	return nil
}
