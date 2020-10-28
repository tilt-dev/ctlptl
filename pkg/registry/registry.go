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
	"github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var typeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "Registry"}
var listTypeMeta = api.TypeMeta{APIVersion: "ctlptl.dev/v1alpha1", Kind: "RegistryList"}
var groupResource = schema.GroupResource{"ctlptl.dev", "registries"}

func TypeMeta() api.TypeMeta {
	return typeMeta
}

func ListTypeMeta() api.TypeMeta {
	return listTypeMeta
}

type ContainerClient interface {
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
}

type Controller struct {
	dockerClient ContainerClient
}

func NewController(dockerClient ContainerClient) (*Controller, error) {
	return &Controller{
		dockerClient: dockerClient,
	}, nil
}

func DefaultController(ctx context.Context) (*Controller, error) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	dockerClient.NegotiateAPIVersion(ctx)

	return NewController(dockerClient)
}

func (c *Controller) Apply(ctx context.Context, registry *api.Registry) (*api.Registry, error) {
	fmt.Printf("Registry Apply is currently a stub! You applied:\n%+v\n", registry)
	return registry, nil
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

		hostPort, containerPort := c.portsFrom(container.Ports)

		registry := &api.Registry{
			TypeMeta: typeMeta,
			Name:     name,
			Status: api.RegistryStatus{
				CreationTimestamp: metav1.Time{Time: created},
				IPAddress:         ipAddress,
				HostPort:          hostPort,
				ContainerPort:     containerPort,
				Networks:          networks,
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

func (c *Controller) portsFrom(ports []types.Port) (hostPort int, containerPort int) {
	for _, port := range ports {
		if port.IP != "0.0.0.0" {
			continue
		}
		if port.PublicPort == 0 {
			continue
		}

		return int(port.PublicPort), int(port.PrivatePort)
	}
	return 0, 0
}
