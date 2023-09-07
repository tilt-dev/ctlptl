package cluster

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/internal/exec"
)

func TestNodeImage(t *testing.T) {
	runner := exec.NewFakeCmdRunner(func(argv []string) string {
		return ""
	})
	iostreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	a := newKindAdmin(iostreams, runner, &fakeDockerClient{})
	ctx := context.Background()

	img, err := a.getNodeImage(ctx, "v0.9.0", "v1.19")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600", img)

	img, err = a.getNodeImage(ctx, "v0.9.0", "v1.19.3")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600", img)

	img, err = a.getNodeImage(ctx, "v0.10.0", "v1.20")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.20.2@sha256:8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab", img)

	img, err = a.getNodeImage(ctx, "v0.11.1", "v1.23")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.23.0@sha256:49824ab1727c04e56a21a5d8372a402fcd32ea51ac96a2706a12af38934f81ac", img)

	img, err = a.getNodeImage(ctx, "v0.8.1", "v1.16.1")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765", img)
}
