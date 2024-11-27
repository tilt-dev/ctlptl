package cluster

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
)

func TestMinikubeStartFlags(t *testing.T) {
	f := newMinikubeFixture()
	ctx := context.Background()
	err := f.a.Create(ctx, &api.Cluster{Name: "minikube", Minikube: &api.MinikubeCluster{StartFlags: []string{"--foo"}}}, nil, []*api.Registry{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"minikube", "start",
		"--foo",
		"-p", "minikube",
		"--driver=docker",
		"--container-runtime=containerd",
		"--extra-config=kubelet.max-pods=500",
	}, f.runner.LastArgs)
}

type minikubeFixture struct {
	runner *exec.FakeCmdRunner
	a      *minikubeAdmin
}

func newMinikubeFixture() *minikubeFixture {
	dockerClient := &fakeDockerClient{ncpu: 1}
	iostreams := genericclioptions.IOStreams{Out: os.Stdout, ErrOut: os.Stderr}
	runner := exec.NewFakeCmdRunner(func(argv []string) string {
		if argv[1] == "version" {
			return `{"commit":"62e108c3dfdec8029a890ad6d8ef96b6461426dc","minikubeVersion":"v1.25.2"}`
		}
		return ""
	})
	return &minikubeFixture{
		runner: runner,
		a:      newMinikubeAdmin(iostreams, dockerClient, runner),
	}
}
