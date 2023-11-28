package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func applyContainerdPatchRegistryApiV2(
	ctx context.Context, runner exec.CmdRunner, iostreams genericclioptions.IOStreams,
	nodes []string, desired *api.Cluster, registry *api.Registry) error {
	for _, node := range nodes {
		contents := fmt.Sprintf(`[host."http://%s:%d"]
`, registry.Name, registry.Status.ContainerPort)

		localRegistryDir := fmt.Sprintf("/etc/containerd/certs.d/localhost:%d", registry.Status.HostPort)
		err := runner.RunIO(ctx,
			genericclioptions.IOStreams{In: strings.NewReader(contents), Out: iostreams.Out, ErrOut: iostreams.ErrOut},
			"docker", "exec", "-i", node, "sh", "-c",
			fmt.Sprintf("mkdir -p %s && cp /dev/stdin %s/hosts.toml", localRegistryDir, localRegistryDir))
		if err != nil {
			return errors.Wrap(err, "configuring registry")
		}

		networkRegistryDir := fmt.Sprintf("/etc/containerd/certs.d/%s:%d", registry.Name, registry.Status.ContainerPort)
		err = runner.RunIO(ctx,
			genericclioptions.IOStreams{In: strings.NewReader(contents), Out: iostreams.Out, ErrOut: iostreams.ErrOut},
			"docker", "exec", "-i", node, "sh", "-c",
			fmt.Sprintf("mkdir -p %s && cp /dev/stdin %s/hosts.toml", networkRegistryDir, networkRegistryDir))
		if err != nil {
			return errors.Wrap(err, "configuring registry")
		}
	}
	return nil
}
