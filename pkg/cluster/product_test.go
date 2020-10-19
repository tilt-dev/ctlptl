package cluster

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/mitchellh/go-homedir"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

type expectedConfig struct {
	expected Product
	input    *api.Context
}

func TestProductFromContext(t *testing.T) {
	minikubeContext := &api.Context{
		Cluster: "minikube",
	}
	minikubePrefixContext := &api.Context{
		Cluster: "minikube-dev-cluster-1",
	}
	dockerDesktopContext := &api.Context{
		Cluster: "docker-for-desktop-cluster",
	}
	dockerDesktopPrefixContext := &api.Context{
		Cluster: "docker-for-desktop-cluster-dev-cluster-1",
	}
	dockerDesktopEdgeContext := &api.Context{
		Cluster: "docker-desktop",
	}
	dockerDesktopEdgePrefixContext := &api.Context{
		Cluster: "docker-desktop-dev-cluster-1",
	}
	gkeContext := &api.Context{
		Cluster: "gke_blorg-dev_us-central1-b_blorg",
	}
	kind5Context := &api.Context{
		Cluster: "kind",
	}
	microK8sContext := &api.Context{
		Cluster: "microk8s-cluster",
	}
	microK8sPrefixContext := &api.Context{
		Cluster: "microk8s-cluster-dev-cluster-1",
	}
	krucibleContext := &api.Context{
		Cluster: "krucible-c-74701fe1a05596b3",
	}
	crcContext := &api.Context{
		Cluster: "api-crc-testing",
	}
	crcPrefixContext := &api.Context{
		Cluster: "api-crc-testing:6443",
	}

	homedir, err := homedir.Dir()
	assert.NoError(t, err)
	k3dContext := &api.Context{
		LocationOfOrigin: filepath.Join(homedir, ".config", "k3d", "k3s-default", "kubeconfig.yaml"),
		Cluster:          "default",
	}
	kind5NamedClusterContext := &api.Context{
		LocationOfOrigin: filepath.Join(homedir, ".kube", "kind-config-integration"),
		Cluster:          "integration",
	}
	kind6Context := &api.Context{
		Cluster: "kind-custom-name",
	}
	table := []expectedConfig{
		{ProductMinikube, minikubeContext},
		{ProductMinikube, minikubePrefixContext},
		{ProductDockerDesktop, dockerDesktopContext},
		{ProductDockerDesktop, dockerDesktopPrefixContext},
		{ProductDockerDesktop, dockerDesktopEdgeContext},
		{ProductDockerDesktop, dockerDesktopEdgePrefixContext},
		{ProductGKE, gkeContext},
		{ProductKIND, kind5Context},
		{ProductMicroK8s, microK8sContext},
		{ProductMicroK8s, microK8sPrefixContext},
		{ProductCRC, crcContext},
		{ProductCRC, crcPrefixContext},
		{ProductKrucible, krucibleContext},
		{ProductK3D, k3dContext},
		{ProductKIND, kind5NamedClusterContext},
		{ProductKIND, kind6Context},
	}

	for i, tt := range table {
		t.Run(fmt.Sprintf("product %d", i), func(t *testing.T) {
			actual := productFromContext(tt.input)
			if actual != tt.expected {
				t.Errorf("Expected %s, actual %s", tt.expected, actual)
			}
		})
	}
}
