package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"github.com/tilt-dev/localregistry-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

var createTime = time.Unix(1500000000, 0)
var startTime = time.Unix(1600000000, 0)
var clusterType = cluster.TypeMeta()
var clusterList = &api.ClusterList{
	TypeMeta: cluster.ListTypeMeta(),
	Items: []api.Cluster{
		api.Cluster{
			TypeMeta: clusterType,
			Name:     "microk8s",
			Product:  "microk8s",
			Status: api.ClusterStatus{
				CreationTimestamp: metav1.Time{Time: createTime},
			},
		},
		api.Cluster{
			TypeMeta: clusterType,
			Name:     "kind-kind",
			Product:  "KIND",
			Status: api.ClusterStatus{
				CreationTimestamp: metav1.Time{Time: createTime},
				LocalRegistryHosting: &localregistry.LocalRegistryHostingV1{
					Host: "localhost:5000",
				},
			},
		},
	},
}

func TestDefaultPrint(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams
	o.StartTime = startTime

	err := o.Print(o.transformForOutput(clusterList))
	require.NoError(t, err)
	assert.Equal(t, out.String(), `NAME        PRODUCT    AGE   REGISTRY
microk8s    microk8s   3y    none
kind-kind   KIND       3y    localhost:5000
`)
}

func TestYAML(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams
	o.StartTime = startTime

	o.Command().Flags().Set("output", "yaml")

	err := o.Print(o.transformForOutput(clusterList))
	require.NoError(t, err)
	assert.Equal(t, `apiVersion: ctlptl.dev/v1alpha1
items:
- apiVersion: ctlptl.dev/v1alpha1
  kind: Cluster
  name: microk8s
  product: microk8s
  status:
    creationTimestamp: "2017-07-14T02:40:00Z"
- apiVersion: ctlptl.dev/v1alpha1
  kind: Cluster
  name: kind-kind
  product: KIND
  status:
    creationTimestamp: "2017-07-14T02:40:00Z"
    localRegistryHosting:
      host: localhost:5000
kind: ClusterList
`, out.String())
}
