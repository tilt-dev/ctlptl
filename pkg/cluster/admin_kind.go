package cluster

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"github.com/tilt-dev/localregistry-go"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	"github.com/tilt-dev/ctlptl/internal/dctr"
	cexec "github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/pkg/api"
)

func kindNetworkName() string {
	networkName := "kind"
	if n := os.Getenv("KIND_EXPERIMENTAL_DOCKER_NETWORK"); n != "" {
		networkName = n
	}
	return networkName
}

// kindAdmin uses the kind CLI to manipulate a kind cluster,
// once the underlying machine has been setup.
type kindAdmin struct {
	iostreams    genericclioptions.IOStreams
	runner       cexec.CmdRunner
	dockerClient dctr.Client
}

func newKindAdmin(iostreams genericclioptions.IOStreams,
	runner cexec.CmdRunner, dockerClient dctr.Client) *kindAdmin {
	return &kindAdmin{
		iostreams:    iostreams,
		runner:       runner,
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

func (a *kindAdmin) kindClusterConfig(desired *api.Cluster, registry *api.Registry, registryAPI containerdRegistryAPI) (*v1alpha4.Cluster, error) {
	kindConfig := desired.KindV1Alpha4Cluster
	if kindConfig == nil {
		kindConfig = &v1alpha4.Cluster{}
	} else {
		kindConfig = kindConfig.DeepCopy()
	}
	kindConfig.Kind = "Cluster"
	kindConfig.APIVersion = "kind.x-k8s.io/v1alpha4"

	if registry != nil {
		if registryAPI == containerdRegistryV2 && len(desired.RegistryAuths) == 0 {
			// Point to the registry config path.
			// We'll add these files post-creation.
			patch := `[plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
`
			kindConfig.ContainerdConfigPatches = append(kindConfig.ContainerdConfigPatches, patch)
		} else {
			patch := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:%d"]
  endpoint = ["http://%s:%d"]
[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s:%d"]
  endpoint = ["http://%s:%d"]
`, registry.Status.HostPort, registry.Name, registry.Status.ContainerPort,
				registry.Name, registry.Status.ContainerPort, registry.Name, registry.Status.ContainerPort)
			kindConfig.ContainerdConfigPatches = append(kindConfig.ContainerdConfigPatches, patch)
		}
	}

	for _, reg := range desired.RegistryAuths {
		// Parse the endpoint
		parsedEndpoint, err := url.Parse(reg.Endpoint)
		if err != nil {
			return nil, errors.Wrapf(err, "Error parsing registry endpoint: %s", reg.Endpoint)
		}

		// Add the registry to the list of mirrors.
		patch := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.mirrors."%s"]
  endpoint = ["%s"]
`, reg.Host, reg.Endpoint)
		kindConfig.ContainerdConfigPatches = append(kindConfig.ContainerdConfigPatches, patch)

		// Specify the auth for the registry, if provided.
		if reg.Username != "" || reg.Password != "" {
			usernameValue := os.ExpandEnv(reg.Username)
			passwordValue := os.ExpandEnv(reg.Password)

			patch := fmt.Sprintf(`[plugins."io.containerd.grpc.v1.cri".registry.configs."%s".auth]
  username = "%s"
  password = "%s"
`, parsedEndpoint.Host, usernameValue, passwordValue)
			kindConfig.ContainerdConfigPatches = append(kindConfig.ContainerdConfigPatches, patch)
		}
	}

	return kindConfig, nil
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

	// If a cluster has been registered with Kind, but deleted from our kubeconfig,
	// Kind will refuse to create a new cluster. The only way to salvage it is
	// to delete and recreate.
	exists, err := a.clusterExists(ctx, kindName)
	if err != nil {
		return err
	}

	if exists {
		klog.V(3).Infof("Deleting orphaned KIND cluster: %s", kindName)
		err := a.runner.RunIO(ctx, a.iostreams, "kind", "delete", "cluster", "--name", kindName)
		if err != nil {
			return errors.Wrap(err, "deleting orphaned kind cluster")
		}
	}

	kindVersion, err := a.getKindVersion(ctx)
	if err != nil {
		return errors.Wrap(err, "creating cluster")
	}

	registryAPI := containerdRegistryV2
	kindVersionSemver, err := semver.ParseTolerant(kindVersion)
	if err != nil {
		return errors.Wrap(err, "parsing kind version")
	}
	if kindVersionSemver.LT(semver.MustParse("0.20.0")) {
		registryAPI = containerdRegistryV1
	}

	args := []string{"create", "cluster", "--name", kindName}
	if desired.KubernetesVersion != "" {
		node, err := a.getNodeImage(ctx, kindVersion, desired.KubernetesVersion)
		if err != nil {
			return errors.Wrap(err, "creating cluster")
		}
		args = append(args, "--image", node)
	}

	kindConfig, err := a.kindClusterConfig(desired, registry, registryAPI)
	if err != nil {
		return errors.Wrap(err, "generating kind config")
	}

	buf := bytes.NewBuffer(nil)
	encoder := yaml.NewEncoder(buf)
	err = encoder.Encode(kindConfig)
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}

	args = append(args, desired.KindExtraCreateArguments...)
	args = append(args, "--config", "-")

	iostreams := a.iostreams
	iostreams.In = buf
	err = a.runner.RunIO(ctx, iostreams, "kind", args...)
	if err != nil {
		return errors.Wrap(err, "creating kind cluster")
	}

	networkName := kindNetworkName()

	if registry != nil {
		if !a.inKindNetwork(registry, networkName) {
			_, _ = fmt.Fprintf(a.iostreams.ErrOut, "   Connecting kind to registry %s\n", registry.Name)
			err := a.dockerClient.NetworkConnect(ctx, networkName, registry.Name, nil)
			if err != nil {
				return errors.Wrap(err, "connecting registry")
			}
		}

		if registryAPI == containerdRegistryV2 && len(desired.RegistryAuths) == 0 {
			err = a.applyContainerdPatchRegistryApiV2(ctx, desired, registry)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// We want to make sure that the image is pullable from either:
// localhost:[registry-port] or
// [registry-name]:5000
// by configuring containerd to rewrite the url.
func (a *kindAdmin) applyContainerdPatchRegistryApiV2(ctx context.Context, desired *api.Cluster, registry *api.Registry) error {
	nodes, err := a.getNodes(ctx, desired.Name)
	if err != nil {
		return errors.Wrap(err, "configuring registry")
	}
	filtered := []string{}
	for _, node := range nodes {
		if strings.HasSuffix(node, "external-load-balancer") {
			// Ignore the external load balancers.
			// These load-balance traffic to the control plane nodes.
			// They don't need registry configuration.
			continue
		}
		filtered = append(filtered, node)
	}

	return applyContainerdPatchRegistryApiV2(ctx, a.runner, a.iostreams,
		filtered, desired, registry)
}

func (a *kindAdmin) getNodes(ctx context.Context, cluster string) ([]string, error) {
	kindName := strings.TrimPrefix(cluster, "kind-")
	buf := bytes.NewBuffer(nil)
	iostreams := a.iostreams
	iostreams.Out = buf
	err := a.runner.RunIO(ctx, iostreams,
		"kind", "get", "nodes", "--name", kindName)
	if err != nil {
		return nil, errors.Wrap(err, "kind get nodes")
	}

	scanner := bufio.NewScanner(buf)
	result := []string{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func (a *kindAdmin) clusterExists(ctx context.Context, cluster string) (bool, error) {
	buf := bytes.NewBuffer(nil)
	iostreams := a.iostreams
	iostreams.Out = buf
	err := a.runner.RunIO(ctx, iostreams, "kind", "get", "clusters")
	if err != nil {
		return false, errors.Wrap(err, "kind get clusters")
	}

	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == cluster {
			return true, nil
		}
	}
	return false, nil
}

func (a *kindAdmin) inKindNetwork(registry *api.Registry, networkName string) bool {
	for _, n := range registry.Status.Networks {
		if n == networkName {
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
	err := a.runner.RunIO(ctx, a.iostreams, "kind", "delete", "cluster", "--name", kindName)
	if err != nil {
		return errors.Wrap(err, "deleting kind cluster")
	}
	return nil
}

func (a *kindAdmin) ModifyConfigInContainer(ctx context.Context, cluster *api.Cluster, containerID string, dockerClient dctr.Client, configWriter configWriter) error {
	err := dockerClient.NetworkConnect(ctx, kindNetworkName(), containerID, nil)
	if err != nil {
		if !errdefs.IsForbidden(err) || !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("error connecting to cluster network: %w", err)
		}
	}

	kindName := strings.TrimPrefix(cluster.Name, "kind-")
	return configWriter.SetConfig(
		fmt.Sprintf("clusters.%s.server", cluster.Name),
		fmt.Sprintf("https://%s-control-plane:6443", kindName),
	)
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
	out := bytes.NewBuffer(nil)
	iostreams := a.iostreams
	iostreams.Out = out
	err := a.runner.RunIO(ctx, iostreams, "kind", "version")
	if err != nil {
		return "", errors.Wrap(err, "kind version")
	}

	parts := strings.Split(out.String(), " ")
	if len(parts) < 2 {
		return "", fmt.Errorf("parsing kind version output: %s", out.String())
	}

	return parts[1], nil
}

// This table must be built up manually from the Kind release notes each
// time a new Kind version is released :\
var kindK8sNodeTable = map[string]map[string]string{
	"v0.26.0": {
		"1.32": "kindest/node:v1.32.0@sha256:c48c62eac5da28cdadcf560d1d8616cfa6783b58f0d94cf63ad1bf49600cb027",
		"1.31": "kindest/node:v1.31.4@sha256:2cb39f7295fe7eafee0842b1052a599a4fb0f8bcf3f83d96c7f4864c357c6c30",
		"1.30": "kindest/node:v1.30.8@sha256:17cd608b3971338d9180b00776cb766c50d0a0b6b904ab4ff52fd3fc5c6369bf",
		"1.29": "kindest/node:v1.29.12@sha256:62c0672ba99a4afd7396512848d6fc382906b8f33349ae68fb1dbfe549f70dec",
	},
	"v0.25.0": {
		"1.31": "kindest/node:v1.31.2@sha256:18fbefc20a7113353c7b75b5c869d7145a6abd6269154825872dc59c1329912e",
		"1.30": "kindest/node:v1.30.6@sha256:b6d08db72079ba5ae1f4a88a09025c0a904af3b52387643c285442afb05ab994",
		"1.29": "kindest/node:v1.29.10@sha256:3b2d8c31753e6c8069d4fc4517264cd20e86fd36220671fb7d0a5855103aa84b",
		"1.28": "kindest/node:v1.28.15@sha256:a7c05c7ae043a0b8c818f5a06188bc2c4098f6cb59ca7d1856df00375d839251",
		"1.27": "kindest/node:v1.27.16@sha256:2d21a61643eafc439905e18705b8186f3296384750a835ad7a005dceb9546d20",
		"1.26": "kindest/node:v1.26.15@sha256:c79602a44b4056d7e48dc20f7504350f1e87530fe953428b792def00bc1076dd",
	},
	"v0.24.0": {
		"1.31": "kindest/node:v1.31.0@sha256:53df588e04085fd41ae12de0c3fe4c72f7013bba32a20e7325357a1ac94ba865",
		"1.30": "kindest/node:v1.30.4@sha256:976ea815844d5fa93be213437e3ff5754cd599b040946b5cca43ca45c2047114",
		"1.29": "kindest/node:v1.29.8@sha256:d46b7aa29567e93b27f7531d258c372e829d7224b25e3fc6ffdefed12476d3aa",
		"1.28": "kindest/node:v1.28.13@sha256:45d319897776e11167e4698f6b14938eb4d52eb381d9e3d7a9086c16c69a8110",
		"1.27": "kindest/node:v1.27.17@sha256:3fd82731af34efe19cd54ea5c25e882985bafa2c9baefe14f8deab1737d9fabe",
		"1.26": "kindest/node:v1.26.15@sha256:1cc15d7b1edd2126ef051e359bf864f37bbcf1568e61be4d2ed1df7a3e87b354",
		"1.25": "kindest/node:v1.25.16@sha256:6110314339b3b44d10da7d27881849a87e092124afab5956f2e10ecdb463b025",
	},
	"v0.23.0": {
		"1.30": "kindest/node:v1.30.0@sha256:047357ac0cfea04663786a612ba1eaba9702bef25227a794b52890dd8bcd692e",
		"1.29": "kindest/node:v1.29.4@sha256:3abb816a5b1061fb15c6e9e60856ec40d56b7b52bcea5f5f1350bc6e2320b6f8",
		"1.28": "kindest/node:v1.28.9@sha256:dca54bc6a6079dd34699d53d7d4ffa2e853e46a20cd12d619a09207e35300bd0",
		"1.27": "kindest/node:v1.27.13@sha256:17439fa5b32290e3ead39ead1250dca1d822d94a10d26f1981756cd51b24b9d8",
		"1.26": "kindest/node:v1.26.15@sha256:84333e26cae1d70361bb7339efb568df1871419f2019c80f9a12b7e2d485fe19",
		"1.25": "kindest/node:v1.25.16@sha256:5da57dfc290ac3599e775e63b8b6c49c0c85d3fec771cd7d55b45fae14b38d3b",
	},
	"v0.22.0": {
		"1.29": "kindest/node:v1.29.2@sha256:51a1434a5397193442f0be2a297b488b6c919ce8a3931be0ce822606ea5ca245",
		"1.28": "kindest/node:v1.28.7@sha256:9bc6c451a289cf96ad0bbaf33d416901de6fd632415b076ab05f5fa7e4f65c58",
		"1.27": "kindest/node:v1.27.11@sha256:681253009e68069b8e01aad36a1e0fa8cf18bb0ab3e5c4069b2e65cafdd70843",
		"1.26": "kindest/node:v1.26.14@sha256:5d548739ddef37b9318c70cb977f57bf3e5015e4552be4e27e57280a8cbb8e4f",
		"1.25": "kindest/node:v1.25.16@sha256:e8b50f8e06b44bb65a93678a65a26248fae585b3d3c2a669e5ca6c90c69dc519",
		"1.24": "kindest/node:v1.24.17@sha256:bad10f9b98d54586cba05a7eaa1b61c6b90bfc4ee174fdc43a7b75ca75c95e51",
		"1.23": "kindest/node:v1.23.17@sha256:14d0a9a892b943866d7e6be119a06871291c517d279aedb816a4b4bc0ec0a5b3",
	},
	"v0.21.0": {
		"1.29": "kindest/node:v1.29.1@sha256:a0cc28af37cf39b019e2b448c54d1a3f789de32536cb5a5db61a49623e527144",
		"1.28": "kindest/node:v1.28.6@sha256:b7e1cf6b2b729f604133c667a6be8aab6f4dde5bb042c1891ae248d9154f665b",
		"1.27": "kindest/node:v1.27.10@sha256:3700c811144e24a6c6181065265f69b9bf0b437c45741017182d7c82b908918f",
		"1.26": "kindest/node:v1.26.13@sha256:15ae92d507b7d4aec6e8920d358fc63d3b980493db191d7327541fbaaed1f789",
		"1.25": "kindest/node:v1.25.16@sha256:9d0a62b55d4fe1e262953be8d406689b947668626a357b5f9d0cfbddbebbc727",
		"1.24": "kindest/node:v1.24.17@sha256:ea292d57ec5dd0e2f3f5a2d77efa246ac883c051ff80e887109fabefbd3125c7",
		"1.23": "kindest/node:v1.23.17@sha256:fbb92ac580fce498473762419df27fa8664dbaa1c5a361b5957e123b4035bdcf",
	},
	"v0.20.0": {
		"1.28": "kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31",
		"1.27": "kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72",
		"1.26": "kindest/node:v1.26.6@sha256:6e2d8b28a5b601defe327b98bd1c2d1930b49e5d8c512e1895099e4504007adb",
		"1.25": "kindest/node:v1.25.11@sha256:227fa11ce74ea76a0474eeefb84cb75d8dad1b08638371ecf0e86259b35be0c8",
		"1.24": "kindest/node:v1.24.15@sha256:7db4f8bea3e14b82d12e044e25e34bd53754b7f2b0e9d56df21774e6f66a70ab",
		"1.23": "kindest/node:v1.23.17@sha256:59c989ff8a517a93127d4a536e7014d28e235fb3529d9fba91b3951d461edfdb",
		"1.22": "kindest/node:v1.22.17@sha256:f5b2e5698c6c9d6d0adc419c0deae21a425c07d81bbf3b6a6834042f25d4fba2",
		"1.21": "kindest/node:v1.21.14@sha256:8a4e9bb3f415d2bb81629ce33ef9c76ba514c14d707f9797a01e3216376ba093",
	},
	"v0.19.0": {
		"1.27": "kindest/node:v1.27.1@sha256:b7d12ed662b873bd8510879c1846e87c7e676a79fefc93e17b2a52989d3ff42b",
		"1.26": "kindest/node:v1.26.4@sha256:f4c0d87be03d6bea69f5e5dc0adb678bb498a190ee5c38422bf751541cebe92e",
		"1.25": "kindest/node:v1.25.9@sha256:c08d6c52820aa42e533b70bce0c2901183326d86dcdcbedecc9343681db45161",
		"1.24": "kindest/node:v1.24.13@sha256:cea86276e698af043af20143f4bf0509e730ec34ed3b7fa790cc0bea091bc5dd",
		"1.23": "kindest/node:v1.23.17@sha256:f77f8cf0b30430ca4128cc7cfafece0c274a118cd0cdb251049664ace0dee4ff",
		"1.22": "kindest/node:v1.22.17@sha256:9af784f45a584f6b28bce2af84c494d947a05bd709151466489008f80a9ce9d5",
		"1.21": "kindest/node:v1.21.14@sha256:220cfafdf6e3915fbce50e13d1655425558cb98872c53f802605aa2fb2d569cf",
	},
	"v0.18.0": {
		"1.27": "kindest/node:v1.27.1@sha256:9915f5629ef4d29f35b478e819249e89cfaffcbfeebda4324e5c01d53d937b09",
		"1.26": "kindest/node:v1.26.3@sha256:61b92f38dff6ccc29969e7aa154d34e38b89443af1a2c14e6cfbd2df6419c66f",
		"1.25": "kindest/node:v1.25.8@sha256:00d3f5314cc35327706776e95b2f8e504198ce59ac545d0200a89e69fce10b7f",
		"1.24": "kindest/node:v1.24.12@sha256:1e12918b8bc3d4253bc08f640a231bb0d3b2c5a9b28aa3f2ca1aee93e1e8db16",
		"1.23": "kindest/node:v1.23.17@sha256:e5fd1d9cd7a9a50939f9c005684df5a6d145e8d695e78463637b79464292e66c",
		"1.22": "kindest/node:v1.22.17@sha256:c8a828709a53c25cbdc0790c8afe12f25538617c7be879083248981945c38693",
		"1.21": "kindest/node:v1.21.14@sha256:27ef72ea623ee879a25fe6f9982690a3e370c68286f4356bf643467c552a3888",
	},
	"v0.17.0": {
		"1.26": "kindest/node:v1.26.0@sha256:691e24bd2417609db7e589e1a479b902d2e209892a10ce375fab60a8407c7352",
		"1.25": "kindest/node:v1.25.3@sha256:f52781bc0d7a19fb6c405c2af83abfeb311f130707a0e219175677e366cc45d1",
		"1.24": "kindest/node:v1.24.7@sha256:577c630ce8e509131eab1aea12c022190978dd2f745aac5eb1fe65c0807eb315",
		"1.23": "kindest/node:v1.23.13@sha256:ef453bb7c79f0e3caba88d2067d4196f427794086a7d0df8df4f019d5e336b61",
		"1.22": "kindest/node:v1.22.15@sha256:7d9708c4b0873f0fe2e171e2b1b7f45ae89482617778c1c875f1053d4cef2e41",
		"1.21": "kindest/node:v1.21.14@sha256:9d9eb5fb26b4fbc0c6d95fa8c790414f9750dd583f5d7cee45d92e8c26670aa1",
		"1.20": "kindest/node:v1.20.15@sha256:a32bf55309294120616886b5338f95dd98a2f7231519c7dedcec32ba29699394",
		"1.19": "kindest/node:v1.19.16@sha256:476cb3269232888437b61deca013832fee41f9f074f9bed79f57e4280f7c48b7",
	},
	"v0.16.0": {
		"1.25": "kindest/node:v1.25.2@sha256:9be91e9e9cdf116809841fc77ebdb8845443c4c72fe5218f3ae9eb57fdb4bace",
		"1.24": "kindest/node:v1.24.6@sha256:97e8d00bc37a7598a0b32d1fabd155a96355c49fa0d4d4790aab0f161bf31be1",
		"1.23": "kindest/node:v1.23.12@sha256:9402cf1330bbd3a0d097d2033fa489b2abe40d479cc5ef47d0b6a6960613148a",
		"1.22": "kindest/node:v1.22.15@sha256:bfd5eaae36849bfb3c1e3b9442f3da17d730718248939d9d547e86bbac5da586",
		"1.21": "kindest/node:v1.21.14@sha256:ad5b7446dd8332439f22a1efdac73670f0da158c00f0a70b45716e7ef3fae20b",
		"1.20": "kindest/node:v1.20.15@sha256:45d0194a8069c46483a0e509088ab9249302af561ebee76a1281a1f08ecb4ed3",
		"1.19": "kindest/node:v1.19.16@sha256:a146f9819fece706b337d34125bbd5cb8ae4d25558427bf2fa3ee8ad231236f2",
	},
	"v0.15.0": {
		"1.25": "kindest/node:v1.25.0@sha256:428aaa17ec82ccde0131cb2d1ca6547d13cf5fdabcc0bbecf749baa935387cbf",
		"1.24": "kindest/node:v1.24.4@sha256:adfaebada924a26c2c9308edd53c6e33b3d4e453782c0063dc0028bdebaddf98",
		"1.23": "kindest/node:v1.23.10@sha256:f047448af6a656fae7bc909e2fab360c18c487ef3edc93f06d78cdfd864b2d12",
		"1.22": "kindest/node:v1.22.13@sha256:4904eda4d6e64b402169797805b8ec01f50133960ad6c19af45173a27eadf959",
		"1.21": "kindest/node:v1.21.14@sha256:f9b4d3d1112f24a7254d2ee296f177f628f9b4c1b32f0006567af11b91c1f301",
		"1.20": "kindest/node:v1.20.15@sha256:d67de8f84143adebe80a07672f370365ec7d23f93dc86866f0e29fa29ce026fe",
		"1.19": "kindest/node:v1.19.16@sha256:707469aac7e6805e52c3bde2a8a8050ce2b15decff60db6c5077ba9975d28b98",
		"1.18": "kindest/node:v1.18.20@sha256:61c9e1698c1cb19c3b1d8151a9135b379657aee23c59bde4a8d87923fcb43a91",
	},
	"v0.14.0": {
		"1.24": "kindest/node:v1.24.0@sha256:0866296e693efe1fed79d5e6c7af8df71fc73ae45e3679af05342239cdc5bc8e",
		"1.23": "kindest/node:v1.23.6@sha256:b1fa224cc6c7ff32455e0b1fd9cbfd3d3bc87ecaa8fcb06961ed1afb3db0f9ae",
		"1.22": "kindest/node:v1.22.9@sha256:8135260b959dfe320206eb36b3aeda9cffcb262f4b44cda6b33f7bb73f453105",
		"1.21": "kindest/node:v1.21.12@sha256:f316b33dd88f8196379f38feb80545ef3ed44d9197dca1bfd48bcb1583210207",
		"1.20": "kindest/node:v1.20.15@sha256:6f2d011dffe182bad80b85f6c00e8ca9d86b5b8922cdf433d53575c4c5212248",
		"1.19": "kindest/node:v1.19.16@sha256:d9c819e8668de8d5030708e484a9fdff44d95ec4675d136ef0a0a584e587f65c",
		"1.18": "kindest/node:v1.18.20@sha256:738cdc23ed4be6cc0b7ea277a2ebcc454c8373d7d8fb991a7fcdbd126188e6d7",
	},
	"v0.13.0": {
		"1.24": "kindest/node:v1.24.0@sha256:406fd86d48eaf4c04c7280cd1d2ca1d61e7d0d61ddef0125cb097bc7b82ed6a1",
		"1.23": "kindest/node:v1.23.6@sha256:1af0f1bee4c3c0fe9b07de5e5d3fafeb2eec7b4e1b268ae89fcab96ec67e8355",
		"1.22": "kindest/node:v1.22.9@sha256:6e57a6b0c493c7d7183a1151acff0bfa44bf37eb668826bf00da5637c55b6d5e",
		"1.21": "kindest/node:v1.21.12@sha256:ae05d44cc636ee961068399ea5123ae421790f472c309900c151a44ee35c3e3e",
		"1.20": "kindest/node:v1.20.15@sha256:a6ce604504db064c5e25921c6c0fffea64507109a1f2a512b1b562ac37d652f3",
		"1.19": "kindest/node:v1.19.16@sha256:dec41184d10deca01a08ea548197b77dc99eeacb56ff3e371af3193c86ca99f4",
		"1.18": "kindest/node:v1.18.20@sha256:38a8726ece5d7867fb0ede63d718d27ce2d41af519ce68be5ae7fcca563537ed",
	},
	"v0.12.0": {
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
	"v0.11.1": {
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
	"v0.11.0": {
		"1.21": "kindest/node:v1.21.1@sha256:fae9a58f17f18f06aeac9772ca8b5ac680ebbed985e266f711d936e91d113bad",
		"1.20": "kindest/node:v1.20.7@sha256:e645428988191fc824529fd0bb5c94244c12401cf5f5ea3bd875eb0a787f0fe9",
		"1.19": "kindest/node:v1.19.11@sha256:7664f21f9cb6ba2264437de0eb3fe99f201db7a3ac72329547ec4373ba5f5911",
		"1.18": "kindest/node:v1.18.19@sha256:530378628c7c518503ade70b1df698b5de5585dcdba4f349328d986b8849b1ee",
		"1.17": "kindest/node:v1.17.17@sha256:c581fbf67f720f70aaabc74b44c2332cc753df262b6c0bca5d26338492470c17",
		"1.16": "kindest/node:v1.16.15@sha256:430c03034cd856c1f1415d3e37faf35a3ea9c5aaa2812117b79e6903d1fc9651",
		"1.15": "kindest/node:v1.15.12@sha256:8d575f056493c7778935dd855ded0e95c48cb2fab90825792e8fc9af61536bf9",
		"1.14": "kindest/node:v1.14.10@sha256:6033e04bcfca7c5f2a9c4ce77551e1abf385bcd2709932ec2f6a9c8c0aff6d4f",
	},
	"v0.10.0": {
		"1.20": "kindest/node:v1.20.2@sha256:8f7ea6e7642c0da54f04a7ee10431549c0257315b3a634f6ef2fecaaedb19bab",
		"1.19": "kindest/node:v1.19.7@sha256:a70639454e97a4b733f9d9b67e12c01f6b0297449d5b9cbbef87473458e26dca",
		"1.18": "kindest/node:v1.18.15@sha256:5c1b980c4d0e0e8e7eb9f36f7df525d079a96169c8a8f20d8bd108c0d0889cc4",
		"1.17": "kindest/node:v1.17.17@sha256:7b6369d27eee99c7a85c48ffd60e11412dc3f373658bc59b7f4d530b7056823e",
		"1.16": "kindest/node:v1.16.15@sha256:c10a63a5bda231c0a379bf91aebf8ad3c79146daca59db816fb963f731852a99",
		"1.15": "kindest/node:v1.15.12@sha256:67181f94f0b3072fb56509107b380e38c55e23bf60e6f052fbd8052d26052fb5",
		"1.14": "kindest/node:v1.14.10@sha256:3fbed72bcac108055e46e7b4091eb6858ad628ec51bf693c21f5ec34578f6180",
	},
	"v0.9.0": {
		"1.19": "kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600",
		"1.18": "kindest/node:v1.18.8@sha256:f4bcc97a0ad6e7abaf3f643d890add7efe6ee4ab90baeb374b4f41a4c95567eb",
		"1.17": "kindest/node:v1.17.11@sha256:5240a7a2c34bf241afb54ac05669f8a46661912eab05705d660971eeb12f6555",
		"1.16": "kindest/node:v1.16.15@sha256:a89c771f7de234e6547d43695c7ab047809ffc71a0c3b65aa54eda051c45ed20",
		"1.15": "kindest/node:v1.15.12@sha256:d9b939055c1e852fe3d86955ee24976cab46cba518abcb8b13ba70917e6547a6",
		"1.14": "kindest/node:v1.14.10@sha256:ce4355398a704fca68006f8a29f37aafb49f8fc2f64ede3ccd0d9198da910146",
		"1.13": "kindest/node:v1.13.12@sha256:1c1a48c2bfcbae4d5f4fa4310b5ed10756facad0b7a2ca93c7a4b5bae5db29f5",
	},
	"v0.8.1": {
		"1.18": "kindest/node:v1.18.2@sha256:7b27a6d0f2517ff88ba444025beae41491b016bc6af573ba467b70c5e8e0d85f",
		"1.17": "kindest/node:v1.17.5@sha256:ab3f9e6ec5ad8840eeb1f76c89bb7948c77bbf76bcebe1a8b59790b8ae9a283a",
		"1.16": "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765",
		"1.15": "kindest/node:v1.15.11@sha256:6cc31f3533deb138792db2c7d1ffc36f7456a06f1db5556ad3b6927641016f50",
		"1.14": "kindest/node:v1.14.10@sha256:6cd43ff41ae9f02bb46c8f455d5323819aec858b99534a290517ebc181b443c6",
		"1.13": "kindest/node:v1.13.12@sha256:214476f1514e47fe3f6f54d0f9e24cfb1e4cda449529791286c7161b7f9c08e7",
		"1.12": "kindest/node:v1.12.10@sha256:faeb82453af2f9373447bb63f50bae02b8020968e0889c7fa308e19b348916cb",
	},
	"v0.8.0": {
		"1.18": "kindest/node:v1.18.2@sha256:7b27a6d0f2517ff88ba444025beae41491b016bc6af573ba467b70c5e8e0d85f",
		"1.17": "kindest/node:v1.17.5@sha256:ab3f9e6ec5ad8840eeb1f76c89bb7948c77bbf76bcebe1a8b59790b8ae9a283a",
		"1.16": "kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765",
		"1.15": "kindest/node:v1.15.11@sha256:6cc31f3533deb138792db2c7d1ffc36f7456a06f1db5556ad3b6927641016f50",
		"1.14": "kindest/node:v1.14.10@sha256:6cd43ff41ae9f02bb46c8f455d5323819aec858b99534a290517ebc181b443c6",
		"1.13": "kindest/node:v1.13.12@sha256:214476f1514e47fe3f6f54d0f9e24cfb1e4cda449529791286c7161b7f9c08e7",
		"1.12": "kindest/node:v1.12.10@sha256:faeb82453af2f9373447bb63f50bae02b8020968e0889c7fa308e19b348916cb",
	},
	"v0.7.0": {
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
