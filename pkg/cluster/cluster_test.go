package cluster

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestClusterGet(t *testing.T) {
	c := newFakeController()
	cluster, err := c.Get(context.Background(), "microk8s")
	assert.NoError(t, err)
	assert.Equal(t, cluster.Name, "microk8s")
	assert.Equal(t, cluster.Product, "microk8s")
}

func TestClusterList(t *testing.T) {
	c := newFakeController()
	clusters, err := c.List(context.Background(), ListOptions{})
	assert.NoError(t, err)
	require.Equal(t, 1, len(clusters))
	assert.Equal(t, "microk8s", clusters[0].Name)
}

func TestClusterListSelectorMatch(t *testing.T) {
	c := newFakeController()
	clusters, err := c.List(context.Background(), ListOptions{FieldSelector: "product=microk8s"})
	assert.NoError(t, err)
	require.Equal(t, 1, len(clusters))
	assert.Equal(t, "microk8s", clusters[0].Name)
}

func TestClusterListSelectorNoMatch(t *testing.T) {
	c := newFakeController()
	clusters, err := c.List(context.Background(), ListOptions{FieldSelector: "product=kind"})
	assert.NoError(t, err)
	assert.Equal(t, 0, len(clusters))
}

func TestClusterGetMissing(t *testing.T) {
	c := newFakeController()
	_, err := c.Get(context.Background(), "dunkees")
	if assert.Error(t, err) {
		assert.True(t, errors.IsNotFound(err))
	}
}

func newFakeController() *Controller {
	return &Controller{
		config: clientcmdapi.Config{
			CurrentContext: "microk8s",
			Contexts: map[string]*api.Context{
				"microk8s": &api.Context{
					Cluster: "microk8s-cluster",
				},
			},
		},
		clients: map[string]kubernetes.Interface{
			"microk8s": fake.NewSimpleClientset(),
		},
	}
}
