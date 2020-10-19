package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestDefaultPrint(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams

	err := o.Print(o.clustersAsResources(o.clusters()))
	require.NoError(t, err)
	assert.Equal(t, out.String(), `NAME        PRODUCT
microk8s    microk8s
kind-kind   KIND
`)
}

func TestYAML(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams

	o.Command().Flags().Set("output", "yaml")

	err := o.Print(o.clustersAsResources(o.clusters()))
	require.NoError(t, err)
	assert.Equal(t, out.String(), `apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
name: microk8s
product: microk8s
---
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
name: kind-kind
product: KIND
`)
}
