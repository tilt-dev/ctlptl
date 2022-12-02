package cluster

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/api/k3dv1alpha4"
)

func TestK3DStartFlagsV4(t *testing.T) {
	f := newK3DFixture()
	f.version = "v4.0.0"

	ctx := context.Background()
	v, err := f.a.version(ctx)
	require.NoError(t, err)
	assert.Equal(t, "4.0.0", v.String())

	err = f.a.Create(ctx, &api.Cluster{
		Name: "k3d-my-cluster",
	}, &api.Registry{Name: "my-reg"})
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"k3d", "cluster", "create", "my-cluster",
		"--registry-use", "my-reg",
	}, f.runner.LastArgs)
}

func TestK3DStartFlagsV5(t *testing.T) {
	f := newK3DFixture()

	ctx := context.Background()
	v, err := f.a.version(ctx)
	require.NoError(t, err)
	assert.Equal(t, "5.4.6", v.String())

	err = f.a.Create(ctx, &api.Cluster{
		Name: "k3d-my-cluster",
		K3D: &api.K3DCluster{
			V1Alpha4Simple: &k3dv1alpha4.SimpleConfig{
				Network: "bar",
			},
		},
	}, &api.Registry{Name: "my-reg"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"k3d", "cluster", "create", "my-cluster",
		"--config", "-",
	}, f.runner.LastArgs)
	assert.Equal(t, f.runner.LastStdin, `kind: Simple
apiVersion: k3d.io/v1alpha4
name: my-cluster
network: bar
registries:
    use:
        - my-reg
`)
}

type k3dFixture struct {
	runner  *exec.FakeCmdRunner
	a       *k3dAdmin
	version string
}

func newK3DFixture() *k3dFixture {
	iostreams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr}
	f := &k3dFixture{
		version: "v5.4.6",
	}
	f.runner = exec.NewFakeCmdRunner(func(argv []string) string {
		if argv[1] == "version" {
			return fmt.Sprintf(`k3d version %s
k3s version v1.24.4-k3s1 (default)
`, f.version)
		}
		return ""
	})
	f.a = newK3DAdmin(iostreams, f.runner)
	return f
}
