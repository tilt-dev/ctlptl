package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestDeleteByName(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	cd := &fakeDeleter{}
	o.clusterDeleter = cd
	err := o.run([]string{"cluster", "kind-kind"})
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind deleted\n", out.String())
	assert.Equal(t, "kind-kind", cd.lastName)
}

func TestDeleteByFile(t *testing.T) {
	streams, in, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	_, _ = in.Write([]byte(`apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
name: kind-kind
`))

	cd := &fakeDeleter{}
	o.clusterDeleter = cd
	o.Filenames = []string{"-"}
	err := o.run([]string{})
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind deleted\n", out.String())
	assert.Equal(t, "kind-kind", cd.lastName)
}

func TestDeleteDefault(t *testing.T) {
	streams, in, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	_, _ = in.Write([]byte(`apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
product: kind
`))

	cd := &fakeDeleter{}
	o.clusterDeleter = cd
	o.Filenames = []string{"-"}
	err := o.run([]string{})
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind deleted\n", out.String())
	assert.Equal(t, "kind-kind", cd.lastName)
}

func TestDeleteNotFound(t *testing.T) {
	streams, _, _, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	cd := &fakeDeleter{nextError: errors.NewNotFound(
		schema.GroupResource{"ctlptl.dev", "clusters"}, "garbage")}
	o.clusterDeleter = cd
	err := o.run([]string{"cluster", "garbage"})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), `clusters.ctlptl.dev "garbage" not found`)
	}
}

func TestDeleteIgnoreNotFound(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewDeleteOptions()
	o.IOStreams = streams

	cd := &fakeDeleter{nextError: errors.NewNotFound(
		schema.GroupResource{"ctlptl.dev", "clusters"}, "garbage")}
	o.clusterDeleter = cd
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
