package registry

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/phayes/freeport"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/ctlptl/internal/exec"
	"github.com/tilt-dev/ctlptl/internal/socat"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/docker"
)

var typeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "Registry"}
var listTypeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "RegistryList"}
var groupResource = schema.GroupResource{Group: "ctlptl.dev", Resource: "registries"}

// https://github.com/moby/moby/blob/v20.10.3/api/types/types.go#L313
const containerStateRunning = "running"

func TypeMeta() api.TypeMeta {
	return typeMeta
}

func ListTypeMeta() api.TypeMeta {
	return listTypeMeta
}

func FillDefaults(registry *api.Registry) {
	// Create a default name if one isn't in the YAML.
	// The default name is determined by the underlying product.
	if registry.Name == "" {
		registry.Name = "ctlptl-registry"
	}
}

type ContainerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerRemove(ctx context.Context, id string, options types.ContainerRemoveOptions) error
}

type socatController interface {
	ConnectRemoteDockerPort(ctx context.Context, port int) error
}

type Controller struct {
	iostreams    genericclioptions.IOStreams
	dockerClient ContainerClient
	runner       exec.CmdRunner
	socat        socatController
}

func NewController(iostreams genericclioptions.IOStreams, dockerClient ContainerClient) (*Controller, error) {
	return &Controller{
		iostreams:    iostreams,
		dockerClient: dockerClient,
		runner:       exec.RealCmdRunner{},
		socat:        socat.NewController(dockerClient),
	}, nil
}

func DefaultController(ctx context.Context, iostreams genericclioptions.IOStreams) (*Controller, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	dockerClient.NegotiateAPIVersion(ctx)

	return NewController(iostreams, dockerClient)
}

func (c *Controller) Get(ctx context.Context, name string) (*api.Registry, error) {
	list, err := c.List(ctx, ListOptions{FieldSelector: fmt.Sprintf("name=%s", name)})
	if err != nil {
		return nil, err
	}

	if len(list.Items) == 0 {
		return nil, errors.NewNotFound(groupResource, name)
	}

	item := list.Items[0]
	return &item, nil
}

func (c *Controller) List(ctx context.Context, options ListOptions) (*api.RegistryList, error) {
	selector, err := fields.ParseSelector(options.FieldSelector)
	if err != nil {
		return nil, err
	}

	filterArgs := filters.NewArgs()
	filterArgs.Add("ancestor", "registry:2") // The registry everyone uses.

	containers, err := c.dockerClient.ContainerList(ctx, types.ContainerListOptions{
		Filters: filterArgs,
		All:     true,
	})
	if err != nil {
		return nil, err
	}

	result := []api.Registry{}
	for _, container := range containers {
		if len(container.Names) == 0 {
			continue
		}
		name := strings.TrimPrefix(container.Names[0], "/")
		created := time.Unix(container.Created, 0)

		netSummary := container.NetworkSettings
		ipAddress := ""
		networks := []string{}
		if netSummary != nil {
			for network := range netSummary.Networks {
				networks = append(networks, network)
			}
			bridge, ok := netSummary.Networks["bridge"]
			if ok && bridge != nil {
				ipAddress = bridge.IPAddress
			}
		}
		sort.Strings(networks)

		listenAddress, hostPort, containerPort := c.ipAndPortsFrom(container.Ports)

		registry := &api.Registry{
			TypeMeta: typeMeta,
			Name:     name,
			Port:     hostPort,
			Status: api.RegistryStatus{
				CreationTimestamp: metav1.Time{Time: created},
				ContainerID:       container.ID,
				IPAddress:         ipAddress,
				HostPort:          hostPort,
				ListenAddress:     listenAddress,
				ContainerPort:     containerPort,
				Networks:          networks,
				State:             container.State,
			},
		}

		if !selector.Matches((*registryFields)(registry)) {
			continue
		}
		result = append(result, *registry)
	}
	return &api.RegistryList{
		TypeMeta: listTypeMeta,
		Items:    result,
	}, nil
}

func (c *Controller) ipAndPortsFrom(ports []types.Port) (listenAddress string, hostPort int, containerPort int) {
	for _, port := range ports {
		if port.PrivatePort == 5000 {
			return port.IP, int(port.PublicPort), int(port.PrivatePort)
		}
	}
	return "unknown", 0, 0
}

// Compare the desired registry against the existing registry, and reconcile
// the two to match.
func (c *Controller) Apply(ctx context.Context, desired *api.Registry) (*api.Registry, error) {
	FillDefaults(desired)
	existing, err := c.Get(ctx, desired.Name)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	if existing == nil {
		existing = &api.Registry{}
	}

	needsDelete := false
	if existing.Port != 0 && desired.Port != 0 && existing.Port != desired.Port {
		// If the port has changed, let's delete the registry and recreate it.
		needsDelete = true
	}
	if existing.Status.State != containerStateRunning {
		// If the registry has died, we need to recreate.
		needsDelete = true
	}
	if needsDelete && existing.Name != "" {
		err = c.Delete(ctx, existing.Name)
		if err != nil {
			return nil, err
		}
		existing = &api.Registry{}
	}

	if existing.Status.ContainerID != "" {
		// If we got to this point, and the container id exists, then the registry is up to date!
		return existing, nil
	}

	hostPort := desired.Port
	if hostPort == 0 {
		freePort, err := freeport.GetFreePort()
		if err != nil {
			return nil, err
		}
		hostPort = freePort
	}

	// keep old behavior as default
	ListenAddress := "127.0.0.1"
	if desired.ListenAddress != "" {
		ListenAddress = desired.ListenAddress
	}

	// explicitly bind to IPv4 to prevent issues with the port forward when connected to a Docker network with IPv6 enabled
	// see https://github.com/docker/for-mac/issues/6015
	portSpec := fmt.Sprintf("%s:%d:5000", ListenAddress, hostPort)

	_, _ = fmt.Fprintf(c.iostreams.ErrOut, "Creating registry %q...\n", desired.Name)
	err = c.runner.Run(ctx, "docker", "run", "-d", "--restart=always", "-p", portSpec, "--name", desired.Name, "registry:2")
	if err != nil {
		return nil, err
	}

	err = c.maybeCreateForwarder(ctx, hostPort)
	if err != nil {
		return nil, err
	}

	return c.Get(ctx, desired.Name)
}

func (c *Controller) maybeCreateForwarder(ctx context.Context, port int) error {
	if docker.IsLocalHost(docker.GetHostEnv()) {
		return nil
	}

	_, _ = fmt.Fprintf(c.iostreams.ErrOut, " 🎮 Env DOCKER_HOST set. Assuming remote Docker and forwarding registry to localhost:%d\n", port)
	return c.socat.ConnectRemoteDockerPort(ctx, port)
}

// Delete the given registry.
func (c *Controller) Delete(ctx context.Context, name string) error {
	registry, err := c.Get(ctx, name)
	if err != nil {
		return err
	}

	cID := registry.Status.ContainerID
	if cID == "" {
		return fmt.Errorf("container not running registry: %s", name)
	}

	return c.dockerClient.ContainerRemove(ctx, registry.Status.ContainerID, types.ContainerRemoveOptions{
		Force: true,
	})
}
