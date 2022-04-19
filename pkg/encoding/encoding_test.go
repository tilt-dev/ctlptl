package encoding

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

func TestParse(t *testing.T) {
	yaml := `
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
name: microk8s
product: microk8s
---
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
name: kind-kind
product: KIND
`
	data, err := ParseStream(strings.NewReader(yaml))
	assert.NoError(t, err)
	require.Equal(t, 2, len(data))
	assert.Equal(t, "microk8s", data[0].(*api.Cluster).Name)
	assert.Equal(t, "kind-kind", data[1].(*api.Cluster).Name)
}

func TestParseTypo(t *testing.T) {
	yaml := `
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
nameTypo: microk8s
product: microk8s
`
	_, err := ParseStream(strings.NewReader(yaml))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "decoding {Cluster ctlptl.dev/v1alpha1}: yaml: unmarshal errors:\n  line 4: field nameTypo not found in type api.Cluster")
	}
}

func TestParseTypoSecondObject(t *testing.T) {
	yaml := `
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
name: microk8s
product: microk8s
---
apiVersion: ctlptl.dev/v1alpha1
kind: Cluster
nameTypo: microk8s
product: microk8s
`
	_, err := ParseStream(strings.NewReader(yaml))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "decoding {Cluster ctlptl.dev/v1alpha1}: yaml: unmarshal errors:\n  line 9: field nameTypo not found in type api.Cluster")
	}
}
