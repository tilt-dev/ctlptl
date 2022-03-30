package cluster

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/pkg/errors"
	"github.com/tilt-dev/localregistry-go"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"github.com/tilt-dev/ctlptl/pkg/api"
)

const kindNetworkName = "kind"

// kindAdmin uses the kind CLI to manipulate a kind cluster,
// once the underlying machine has been setup.
type kindAdmin struct {
	iostreams    genericclioptions.IOStreams
	dockerClient dockerClient
}

func newKindAdmin(iostreams genericclioptions.IOStreams, dockerClient dockerClient) *kindAdmin {
	return &kindAdmin{
		iostreams:    iostreams,
		dockerClient: dockerClient,
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
	} else {
		kindConfig = kindConfig.DeepCopy()
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
		err := a.dockerClient.NetworkConnect(ctx, kindNetworkName, registry.Name, nil)
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
		return "", fmt.Errorf("unsupported Kind version %s.\n"+
			"To set up a specific Kubernetes version in Kind, ctlptl needs an official Kubernetes image.\n"+
			"If you're running an unofficial version of Kind, remove 'kubernetesVersion' from your cluster config to use the default image.\n"+
			"If you're running a newly released version of Kind, please file an issue: https://github.com/tilt-dev/ctlptl/issues/new", kindVersion)
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
	"v0.12.0": map[string]string{
		"1.23": "kindest/node:v1.23.4@sha256:0e34f0d0fd448aa2f2819cfd74e99fe5793a6e4938b328f657c8e3f81ee0dfb9",
		"1.22": "kindest/node:v1.22.7@sha256:1dfd72d193bf7da64765fd2f2898f78663b9ba366c2aa74be1fd7498a1873166",
		"1.21": "kindest/node:v1.21.10@sha256:84709f09756ba4f863769bdcabe5edafc2ada72d3c8c44d6515fc581b66b029c",
		"1.20": "kindest/node:v1.20.15@sha256:393bb9096c6c4d723bb17bceb0896407d7db581532d11ea2839c80b28e5d8deb",
		"1.19": "kindest/node:v1.19.16@sha256:81f552397c1e6c1f293f967ecb1344d8857613fb978f963c30e907c32f598467",
		"1.18": "kindest/node:v1.18.20@sha256:e3dca5e16116d11363e31639640042a9b1bd2c90f85717a7fc66be34089a8169",
		"1.17": "kindest/node:v1.17.17@sha256:e477ee64df5731aa4ef4deabbafc34e8d9a686b49178f726563598344a3898d5",
		"1.16": "kindest/node:v1.16.15@sha256:64bac16b83b6adfd04ea3fbcf6c9b5b893277120f2b2cbf9f5fa3e5d4c2260cc",
		"1.15": "kindest/node:v1.15.12@sha256:9dfc13db6d3fd5e5b275f8c4657ee6a62ef9cb405546664f2de2eabcfd6db778",
		"1.14": "kindest/node:v1.14.10@sha256:b693339da2a927949025869425e20daf80111ccabf020d4021a23c00bae29d82",
	},
	"v0.11.1": map[string]string{
		"1.23": "kindest/node:v1.23.0@sha256:49824ab1727c04e56a21a5d8372a402fcd32ea51ac96a2706a12af38934f81ac",
		"1.22": "kindest/node:v1.22.0@sha256:b8bda84bb3a190e6e028b1760d277454a72267a5454b57db34437c34a588d047",
		"1.21": "kindest/node:v1.21.1@sha256:69860bda5563ac81e3c0057d654b5253219618a22ec3a346306239bba8cfa1a6",
		"1.20": "kindest/node:v1.20.7@sha256:cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9",
		"1.19": "kindest/node:v1.19.11@sha256:07db187ae84b4b7de440a73886f008cf903fcf5764ba8106a9fd5243d6f32729",
		"1.18": "kindest/node:v1.18.19@sha256:7af1492e19b3192a79f606e43c35fb741e520d195f96399284515f077b3b622c",
		"1.17": "kindest/node:v1.17.17@sha256:66f1d0d91a88b8a001811e2f1054af60eef3b669a9a74f9b6db871f2f1eeed00",
		"1.16": "kindest/node:v1.16.15@sha256:83067ed51bf2a3395b24687094e283a7c7c865ccc12a8b1d7aa673ba0c5e8861",
		"1.15": "kindest/node:v1.15.12@sha256:b920920e1eda689d9936dfcf7332701e80be12566999152626b2c9d730397a95",
		"1.14": "kindest/node:v1.14.10@sha256:f8a66ef82822ab4f7569e91a5bccaf27bceee135c1457c512e54de8c6f7219f8",
	},
	"v0.11.0": map[string]string{
		"1.21": "kindest/node:v1.21.1@sha256:fae9a58f17f18f06aeac9772ca8b5ac680ebbed985e266f711d936e91d113bad",
		"1.20": "kindest/node:v1.20.7@sha256:e645428988191fc824529fd0bb5c94244c12401cf5f5ea3bd875eb0a787f0fe9",
		"1.19": "kindest/node:v1.19.11@sha256:7664f21f9cb6ba2264437de0eb3fe99f201db7a3ac72329547ec4373ba5f5911",
		"1.18": "kindest/node:v1.18.19@sha256:530378628c7c518503ade70b1df698b5de5585dcdba4f349328d986b8849b1ee",
		"1.17": "kindest/node:v1.17.17@sha256:c581fbf67f720f70aaabc74b44c2332cc753df262b6c0bca5d26338492470c17",
		"1.16": "kindest/node:v1.16.15@sha256:430c03034cd856c1f1415d3e37faf35a3ea9c5aaa2812117b79e6903d1fc9651",
		"1.15": "kindest/node:v1.15.12@sha256:8d575f056493c7778935dd855ded0e95c48cb2fab90825792e8fc9af61536bf9",
		"1.14": "kindest/node:v1.14.10@sha256:6033e04bcfca7c5f2a9c4ce77551e1abf385bcd2709932ec2f6a9c8c0aff6d4f",
	},
	"v0.10.0": map[string]string{
		"1.20": "kindest/node:v1.20.2@sha256:8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab",
		"1.19": "kindest/node:v1.19.7@sha256:a70639454e97a4b733f9d9b67e12c01f6b0297449d5b9cbbef87473458e26dca",
		"1.18": "kindest/node:v1.18.15@sha256:5c1b980c4d0e0e8e7eb9f36f7df525d079a96169c8a8f20d8bd108c0d0889cc4",
		"1.17": "kindest/node:v1.17.17@sha256:7b6369d27eee99c7a85c48ffd60e11412dc3f373658bc59b7f4d530b7056823e",
		"1.16": "kindest/node:v1.16.15@sha256:c10a63a5bda231c0a379bf91aebf8ad3c79146daca59db816fb963f731852a99",
		"1.15": "kindest/node:v1.15.12@sha256:67181f94f0b3072fb56509107b380e38c55e23bf60e6f052fbd8052d26052fb5",
		"1.14": "kindest/node:v1.14.10@sha256:3fbed72bcac108055e46e7b4091eb6858ad628ec51bf693c21f5ec34578f6180",
	},
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
