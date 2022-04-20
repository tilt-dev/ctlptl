package cluster

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestMinikubeStartFlags(t *testing.T) {
	f := newMinikubeFixture()
	ctx := context.Background()
	err := f.a.Create(ctx, &api.Cluster{Name: "minikube", Minikube: &api.MinikubeCluster{StartFlags: []string{"--foo"}}}, nil)
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
	runner := exec.NewFakeCmdRunner(func(argv []string) {})
	return &minikubeFixture{
		runner: runner,
		a:      newMinikubeAdmin(iostreams, dockerClient, runner),
	}
}
