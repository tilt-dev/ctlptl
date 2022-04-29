package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

func TestCreateCluster(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewCreateClusterOptions()
	o.IOStreams = streams

	fcc := &fakeClusterController{}
	err := o.run(fcc, "kind")
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind created\n", out.String())
	assert.Equal(t, "kind-kind", fcc.lastApplyName)
}

type fakeClusterController struct {
	clusters       map[string]*api.Cluster
	lastApplyName  string
	lastDeleteName string
	nextError      error
}

func (cd *fakeClusterController) Delete(ctx context.Context, name string) error {
	if cd.nextError != nil {
		return cd.nextError
	}
	cd.lastDeleteName = name
	delete(cd.clusters, name)
	return nil
}

func (cd *fakeClusterController) Apply(ctx context.Context, cluster *api.Cluster) (*api.Cluster, error) {
	cd.lastApplyName = cluster.Name
	if cd.clusters == nil {
		cd.clusters = make(map[string]*api.Cluster)
	}
	cd.clusters[cluster.Name] = cluster
	return cluster, nil
}

func (cd *fakeClusterController) Get(ctx context.Context, name string) (*api.Cluster, error) {
	cluster, ok := cd.clusters[name]
	if ok {
		return cluster, nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "ctlptl.dev", Resource: "clusters"}, name)
}
