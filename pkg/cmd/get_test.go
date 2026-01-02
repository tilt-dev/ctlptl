package cmd

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/localregistry-go"

	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/cluster"
	"github.com/tilt-dev/ctlptl/pkg/registry"
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
				Current:           true,
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

var registryType = registry.TypeMeta()
var registryList = &api.RegistryList{
	TypeMeta: registry.ListTypeMeta(),
	Items: []api.Registry{
		api.Registry{
			TypeMeta:      registryType,
			Name:          "ctlptl-registry",
			ListenAddress: "127.0.0.1",
			Port:          5001,
			Status: api.RegistryStatus{
				CreationTimestamp: metav1.Time{Time: createTime},
				IPAddress:         "172.17.0.2",
				ListenAddress:     "0.0.0.0",
				ContainerPort:     5000,
				HostPort:          5001,
			},
		},
		api.Registry{
			TypeMeta:      registryType,
			Name:          "ctlptl-registry-loopback",
			ListenAddress: "127.0.0.1",
			Port:          5002,
			Status: api.RegistryStatus{
				CreationTimestamp: metav1.Time{Time: createTime},
				IPAddress:         "172.17.0.3",
				ListenAddress:     "127.0.0.1",
				ContainerPort:     5000,
				HostPort:          5002,
			},
		},
	},
}

func TestDefaultPrint(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams
	o.StartTime = startTime

	err := o.Print(o.toTable(clusterList))
	require.NoError(t, err)
	assert.Equal(t, out.String(), `CURRENT   NAME        PRODUCT    AGE   REGISTRY
*         microk8s    microk8s   3y    none
          kind-kind   KIND       3y    localhost:5000
`)
}

func TestYAML(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams
	o.StartTime = startTime

	err := o.Command().Flags().Set("output", "yaml")
	require.NoError(t, err)

	err = o.Print(clusterList)
	require.NoError(t, err)
	assert.Equal(t, `apiVersion: ctlptl.dev/v1alpha1
items:
- apiVersion: ctlptl.dev/v1alpha1
  kind: Cluster
  name: microk8s
  product: microk8s
  status:
    creationTimestamp: "2017-07-14T02:40:00Z"
    current: true
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

func TestRegistryPrint(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewGetOptions()
	o.IOStreams = streams
	o.StartTime = startTime

	err := o.Print(o.toTable(registryList))
	require.NoError(t, err)
	assert.Equal(t, `NAME                       HOST ADDRESS     CONTAINER ADDRESS   AGE
ctlptl-registry            0.0.0.0:5001     172.17.0.2:5000     3y
ctlptl-registry-loopback   127.0.0.1:5002   172.17.0.3:5000     3y
`, out.String())
}
