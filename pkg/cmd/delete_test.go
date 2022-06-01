package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

func TestDeleteByName(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	cd := &fakeClusterController{}
	o.clusterController = cd
	err := o.run([]string{"cluster", "kind-kind"})
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind deleted\n", out.String())
	assert.Equal(t, "kind-kind", cd.lastDeleteName)
}

func TestDeleteByFile(t *testing.T) {
	streams, in, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	_, _ = in.Write([]byte(`apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
name: kind-kind
`))

	cd := &fakeClusterController{}
	o.clusterController = cd
	o.Filenames = []string{"-"}
	err := o.run([]string{})
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind deleted\n", out.String())
	assert.Equal(t, "kind-kind", cd.lastDeleteName)
}

func TestDeleteDefault(t *testing.T) {
	streams, in, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	_, _ = in.Write([]byte(`apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: kind
`))

	cd := &fakeClusterController{}
	o.clusterController = cd
	o.Filenames = []string{"-"}
	err := o.run([]string{})
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind deleted\n", out.String())
	assert.Equal(t, "kind-kind", cd.lastDeleteName)
}

func TestDeleteNotFound(t *testing.T) {
	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	cd := &fakeClusterController{nextError: errors.NewNotFound(
		schema.GroupResource{Group: "ctlptl.dev", Resource: "clusters"}, "garbage")}
	o.clusterController = cd
	err := o.run([]string{"cluster", "garbage"})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), `clusters.ctlptl.dev "garbage" not found`)
	}
}

func TestDeleteIgnoreNotFound(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	cd := &fakeClusterController{nextError: errors.NewNotFound(
		schema.GroupResource{Group: "ctlptl.dev", Resource: "clusters"}, "garbage")}
	o.clusterController = cd
	o.IgnoreNotFound = true
	err := o.run([]string{"cluster", "garbage"})
	require.NoError(t, err)
	assert.Equal(t, "", out.String())
}

func TestDeleteRegistryByFile(t *testing.T) {
	streams, in, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	_, _ = in.Write([]byte(`apiVersion: ctlptl.dev/v1alpha1
kind: Registry
port: 5002
`))

	rd := &fakeDeleter{}
	o.registryDeleter = rd
	o.Filenames = []string{"-"}
	err := o.run([]string{})
	require.NoError(t, err)
	assert.Equal(t, "registry.ctlptl.dev/ctlptl-registry deleted\n", out.String())
	assert.Equal(t, "ctlptl-registry", rd.lastName)
}

func TestDeleteCascade(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	rd := &fakeDeleter{}
	cd := &fakeClusterController{
		clusters: map[string]*api.Cluster{
			"kind-kind": &api.Cluster{
				Name:     "kind-kind",
				Registry: "my-registry",
			},
		},
	}
	o.clusterController = cd
	o.registryDeleter = rd
	o.Cascade = "true"
	err := o.run([]string{"cluster", "kind-kind"})
	require.NoError(t, err)
	assert.Equal(t,
		"registry.ctlptl.dev/my-registry deleted\n"+
			"cluster.ctlptl.dev/kind-kind deleted\n",
		out.String())
	assert.Equal(t, "my-registry", rd.lastName)
}

func TestDeleteCascadeInvalid(t *testing.T) {
	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	o.Cascade = "xxx"
	err := o.run([]string{"cluster", "kind-kind"})
	if assert.Error(t, err) {
		require.Contains(t, err.Error(), "Invalid cascade: xxx. Valid values: true, false.")
	}
}

type fakeDeleter struct {
	lastName  string
	nextError error
}

func (cd *fakeDeleter) Delete(ctx context.Context, name string) error {
	if cd.nextError != nil {
		return cd.nextError
	}
	cd.lastName = name
	return nil
}
