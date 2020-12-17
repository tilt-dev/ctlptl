package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/ctlptl/pkg/api"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestCreateRegistry(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewCreateRegistryOptions()
	o.IOStreams = streams

	frc := &fakeRegistryController{}
	err := o.run(frc, "my-registry")
	require.NoError(t, err)
	assert.Equal(t, "registry.ctlptl.dev/my-registry created\n", out.String())
	assert.Equal(t, "my-registry", frc.lastRegistry.Name)
}

type fakeRegistryController struct {
	lastRegistry *api.Registry
}

func (cd *fakeRegistryController) Apply(ctx context.Context, registry *api.Registry) (*api.Registry, error) {
	cd.lastRegistry = registry
	return registry, nil
}

func (cd *fakeRegistryController) Get(ctx context.Context, name string) (*api.Registry, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "ctlptl.dev", Resource: "registries"}, name)
}
