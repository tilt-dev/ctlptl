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

func TestCreateCluster(t *testing.T) {
	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	o := NewCreateClusterOptions()
	o.IOStreams = streams

	fcc := &fakeClusterController{}
	err := o.run(fcc, "kind")
	require.NoError(t, err)
	assert.Equal(t, "cluster.ctlptl.dev/kind-kind created\n", out.String())
	assert.Equal(t, "kind-kind", fcc.lastCluster.Name)
}

type fakeClusterController struct {
	lastCluster *api.Cluster
}

func (cd *fakeClusterController) Apply(ctx context.Context, cluster *api.Cluster) (*api.Cluster, error) {
	cd.lastCluster = cluster
	return cluster, nil
}

func (cd *fakeClusterController) Get(ctx context.Context, name string) (*api.Cluster, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{"ctlptl.dev", "clusters"}, name)
}
