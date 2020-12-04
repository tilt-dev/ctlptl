package cluster

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestNodeImage(t *testing.T) {
	iostreams := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
	a := newKindAdmin(iostreams)
	ctx := context.Background()

	img, err := a.getNodeImage(ctx, "v0.9.0", "v1.19")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600", img)

	img, err = a.getNodeImage(ctx, "v0.9.0", "v1.19.3")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600", img)

	img, err = a.getNodeImage(ctx, "v0.8.1", "v1.16.1")
	assert.NoError(t, err)
	assert.Equal(t, "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765", img)
}
