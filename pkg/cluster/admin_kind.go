package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/localregistry-go"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

const kindNetworkName = "kind"

// kindAdmin uses the kind CLI to manipulate a kind cluster,
// once the underlying machine has been setup.
type kindAdmin struct {
	iostreams genericclioptions.IOStreams
}

func newKindAdmin(iostreams genericclioptions.IOStreams) *kindAdmin {
	return &kindAdmin{
		iostreams: iostreams,
	}
}

func (a *kindAdmin) EnsureInstalled(ctx context.Context) error {
	_, err := exec.LookPath("kind")
	if err != nil {
		return fmt.Errorf("kind not installed. Please install kind with these instructions: https://kind.sigs.k8s.io/")
	}
	return nil
}

func (a *kindAdmin) kindClusterConfig(desired *api.Cluster, registry *api.Registry) *v1alpha4.Cluster {
	kindConfig := desired.KindV1Alpha4Cluster
	if kindConfig == nil {
		kindConfig = &v1alpha4.Cluster{}
	}
	kindConfig.Kind = "Cluster"
	kindConfig.APIVersion = "kind.x-k8s.io/v1alpha4"

	if registry != nil {
		patch := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:%d"]
  endpoint = ["http://%s:%d"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s:%d"]
  endpoint = ["http://%s:%d"]
`, registry.Status.HostPort, registry.Name, registry.Status.ContainerPort,
			registry.Name, registry.Status.ContainerPort, registry.Name, registry.Status.ContainerPort)
		kindConfig.ContainerdConfigPatches = append(kindConfig.ContainerdConfigPatches, patch)
	}
	return kindConfig
}

func (a *kindAdmin) Create(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	klog.V(3).Infof("Creating cluster with config:\n%+v\n---\n", desired)
	if registry != nil {
		klog.V(3).Infof("Initializing cluster with registry config:\n%+v\n---\n", registry)
	}

	clusterName := desired.Name
	if !strings.HasPrefix(clusterName, "kind-") {
		return fmt.Errorf("all kind clusters must have a name with the prefix kind-*")
	}

	kindName := strings.TrimPrefix(clusterName, "kind-")

	args := []string{"create", "cluster", "--name", kindName}
	if desired.KubernetesVersion != "" {
		kindVersion, err := a.getKindVersion(ctx)
		if err != nil {
			return errors.Wrap(err, "creating cluster")
		}

		node, err := a.getNodeImage(ctx, kindVersion, desired.KubernetesVersion)
		if err != nil {
			return errors.Wrap(err, "creating cluster")
		}
		args = append(args, "--image", node)
	}

	kindConfig := a.kindClusterConfig(desired, registry)
	buf := bytes.NewBuffer(nil)
	encoder := yaml.NewEncoder(buf)
	err := encoder.Encode(kindConfig)
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}

	args = append(args, "--config", "-")

	cmd := exec.CommandContext(ctx, "kind", args...)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = buf
	err = cmd.Run()
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}

	if registry != nil && !a.inKindNetwork(registry) {
		_, _ = fmt.Fprintf(a.iostreams.ErrOut, "   Connecting kind to registry %s\n", registry.Name)
		cmd := exec.CommandContext(ctx, "docker", "network", "connect", kindNetworkName, registry.Name)
		err := cmd.Run()
		if err != nil {
			return errors.Wrap(err, "connecting registry")
		}
	}

	return nil
}

func (a *kindAdmin) inKindNetwork(registry *api.Registry) bool {
	for _, n := range registry.Status.Networks {
		if n == kindNetworkName {
			return true
		}
	}
	return false
}

func (a *kindAdmin) LocalRegistryHosting(ctx context.Context, desired *api.Cluster, registry *api.Registry) (*localregistry.LocalRegistryHostingV1, error) {
	return &localregistry.LocalRegistryHostingV1{
		Host:                   fmt.Sprintf("localhost:%d", registry.Status.HostPort),
		HostFromClusterNetwork: fmt.Sprintf("%s:%d", registry.Name, registry.Status.ContainerPort),
		Help:                   "https://github.com/tilt-dev/ctlptl",
	}, nil
}

func (a *kindAdmin) Delete(ctx context.Context, config *api.Cluster) error {
	clusterName := config.Name
	if !strings.HasPrefix(clusterName, "kind-") {
		return fmt.Errorf("all kind clusters must have a name with the prefix kind-*")
	}

	kindName := strings.TrimPrefix(clusterName, "kind-")
	cmd := exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", kindName)
	cmd.Stdout = a.iostreams.Out
	cmd.Stderr = a.iostreams.ErrOut
	cmd.Stdin = a.iostreams.In
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "deleting kind cluster")
	}
	return nil
}

func (a *kindAdmin) getNodeImage(ctx context.Context, kindVersion, k8sVersion string) (string, error) {
	nodeTable, ok := kindK8sNodeTable[kindVersion]
	if !ok {
		return "", fmt.Errorf("No available kindest/node versions for kind version %s.\n"+
			"Please file an issue: https://github.com/tilt-dev/ctlptl/issues/new", kindVersion)
	}

	// Kind doesn't maintain Kubernetes nodes for every patch version, so just get the closest
	// major/minor patch.
	k8sVersionParsed, err := semver.ParseTolerant(k8sVersion)
	if err != nil {
		return "", fmt.Errorf("parsing kubernetesVersion: %v", err)
	}

	simplifiedK8sVersion := fmt.Sprintf("%d.%d", k8sVersionParsed.Major, k8sVersionParsed.Minor)
	node, ok := nodeTable[simplifiedK8sVersion]
	if !ok {
		return "", fmt.Errorf("Kind %s does not support Kubernetes v%s", kindVersion, simplifiedK8sVersion)
	}
	return node, nil
}

func (a *kindAdmin) getKindVersion(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "kind", "version")
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "kind version")
	}

	parts := strings.Split(string(out), " ")
	if len(parts) < 2 {
		return "", fmt.Errorf("parsing kind version output: %s", string(out))
	}

	return parts[1], nil
}

// This table must be built up manually from the Kind release notes each
// time a new Kind version is released :\
var kindK8sNodeTable = map[string]map[string]string{
	"v0.9.0": map[string]string{
		"1.19": "kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600",
		"1.18": "kindest/node:v1.18.8@sha256:f4bcc97a0ad6e7abaf3f643d890add7efe6ee4ab90baeb374b4f41a4c95567eb",
		"1.17": "kindest/node:v1.17.11@sha256:5240a7a2c34bf241afb54ac05669f8a46661912eab05705d660971eeb12f6555",
		"1.16": "kindest/node:v1.16.15@sha256:a89c771f7de234e6547d43695c7ab047809ffc71a0c3b65aa54eda051c45ed20",
		"1.15": "kindest/node:v1.15.12@sha256:d9b939055c1e852fe3d86955ee24976cab46cba518abcb8b13ba70917e6547a6",
		"1.14": "kindest/node:v1.14.10@sha256:ce4355398a704fca68006f8a29f37aafb49f8fc2f64ede3ccd0d9198da910146",
		"1.13": "kindest/node:v1.13.12@sha256:1c1a48c2bfcbae4d5f4fa4310b5ed10756facad0b7a2ca93c7a4b5bae5db29f5",
	},
	"v0.8.1": map[string]string{
		"1.18": "kindest/node:v1.18.2@sha256:7b27a6d0f2517ff88ba444025beae41491b016bc6af573ba467b70c5e8e0d85f",
		"1.17": "kindest/node:v1.17.5@sha256:ab3f9e6ec5ad8840eeb1f76c89bb7948c77bbf76bcebe1a8b59790b8ae9a283a",
		"1.16": "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765",
		"1.15": "kindest/node:v1.15.11@sha256:6cc31f3533deb138792db2c7d1ffc36f7456a06f1db5556ad3b6927641016f50",
		"1.14": "kindest/node:v1.14.10@sha256:6cd43ff41ae9f02bb46c8f455d5323819aec858b99534a290517ebc181b443c6",
		"1.13": "kindest/node:v1.13.12@sha256:214476f1514e47fe3f6f54d0f9e24cfb1e4cda449529791286c7161b7f9c08e7",
		"1.12": "kindest/node:v1.12.10@sha256:faeb82453af2f9373447bb63f50bae02b8020968e0889c7fa308e19b348916cb",
	},
	"v0.8.0": map[string]string{
		"1.18": "kindest/node:v1.18.2@sha256:7b27a6d0f2517ff88ba444025beae41491b016bc6af573ba467b70c5e8e0d85f",
		"1.17": "kindest/node:v1.17.5@sha256:ab3f9e6ec5ad8840eeb1f76c89bb7948c77bbf76bcebe1a8b59790b8ae9a283a",
		"1.16": "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765",
		"1.15": "kindest/node:v1.15.11@sha256:6cc31f3533deb138792db2c7d1ffc36f7456a06f1db5556ad3b6927641016f50",
		"1.14": "kindest/node:v1.14.10@sha256:6cd43ff41ae9f02bb46c8f455d5323819aec858b99534a290517ebc181b443c6",
		"1.13": "kindest/node:v1.13.12@sha256:214476f1514e47fe3f6f54d0f9e24cfb1e4cda449529791286c7161b7f9c08e7",
		"1.12": "kindest/node:v1.12.10@sha256:faeb82453af2f9373447bb63f50bae02b8020968e0889c7fa308e19b348916cb",
	},
	"v0.7.0": map[string]string{
		"1.18": "kindest/node:v1.18.0@sha256:0e20578828edd939d25eb98496a685c76c98d54084932f76069f886ec315d694",
		"1.17": "kindest/node:v1.17.0@sha256:9512edae126da271b66b990b6fff768fbb7cd786c7d39e86bdf55906352fdf62",
		"1.16": "kindest/node:v1.16.4@sha256:b91a2c2317a000f3a783489dfb755064177dbc3a0b2f4147d50f04825d016f55",
		"1.15": "kindest/node:v1.15.7@sha256:e2df133f80ef633c53c0200114fce2ed5e1f6947477dbc83261a6a921169488d",
		"1.14": "kindest/node:v1.14.10@sha256:81ae5a3237c779efc4dda43cc81c696f88a194abcc4f8fa34f86cf674aa14977",
		"1.13": "kindest/node:v1.13.12@sha256:5e8ae1a4e39f3d151d420ef912e18368745a2ede6d20ea87506920cd947a7e3a",
		"1.12": "kindest/node:v1.12.10@sha256:68a6581f64b54994b824708286fafc37f1227b7b54cbb8865182ce1e036ed1cc",
		"1.11": "kindest/node:v1.11.10@sha256:e6f3dade95b7cb74081c5b9f3291aaaa6026a90a977e0b990778b6adc9ea6248",
	},
}
