package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"gotest.tools/assert"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var clusterType = api.TypeMeta{Kind: "Cluster", APIVersion: "ctlptl.dev/v1alpha1"}
var clusters = []*api.Cluster{
	&api.Cluster{TypeMeta: clusterType, Name: "microk8s", Product: "microk8s"},
	&api.Cluster{TypeMeta: clusterType, Name: "kind-kind", Product: "KIND"},
}

func TestDefaultPrint(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams

	err := o.Print(o.clustersAsResources(clusters))
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

	err := o.Print(o.clustersAsResources(clusters))
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
