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
	expected     Product
	input        *api.Context
	inputCluster *api.Cluster
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
	k3d3xContext := &api.Context{
		Cluster: "k3d-k3s-default",
	}
	rancherDesktopContext := &api.Context{Cluster: "rancher-desktop"}
	minikubeRandomName := &api.Context{
		Cluster: "custom-name",
	}
	defaultCluster := &api.Cluster{}
	minikubeRandomNameCluster := &api.Cluster{
		CertificateAuthority: filepath.Join(homedir, ".minikube", "ca.crt"),
	}
	table := []expectedConfig{
		{ProductMinikube, minikubeContext, defaultCluster},
		{ProductMinikube, minikubePrefixContext, defaultCluster},
		{ProductDockerDesktop, dockerDesktopContext, defaultCluster},
		{ProductDockerDesktop, dockerDesktopPrefixContext, defaultCluster},
		{ProductDockerDesktop, dockerDesktopEdgeContext, defaultCluster},
		{ProductDockerDesktop, dockerDesktopEdgePrefixContext, defaultCluster},
		{ProductGKE, gkeContext, defaultCluster},
		{ProductKIND, kind5Context, defaultCluster},
		{ProductMicroK8s, microK8sContext, defaultCluster},
		{ProductMicroK8s, microK8sPrefixContext, defaultCluster},
		{ProductCRC, crcContext, defaultCluster},
		{ProductCRC, crcPrefixContext, defaultCluster},
		{ProductKrucible, krucibleContext, defaultCluster},
		{ProductK3D, k3dContext, defaultCluster},
		{ProductKIND, kind5NamedClusterContext, defaultCluster},
		{ProductKIND, kind6Context, defaultCluster},
		{ProductUnknown, minikubeRandomName, defaultCluster},
		{ProductMinikube, minikubeRandomName, minikubeRandomNameCluster},
		{ProductK3D, k3d3xContext, defaultCluster},
		{ProductRancherDesktop, rancherDesktopContext, defaultCluster},
	}

	for i, tt := range table {
		t.Run(fmt.Sprintf("product %d", i), func(t *testing.T) {
			actual := productFromContext(tt.input, tt.inputCluster)
			if actual != tt.expected {
				t.Errorf("Expected %s, actual %s", tt.expected, actual)
			}
		})
	}
}
