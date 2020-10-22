package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/client"
	"github.com/mitchellh/go-homedir"
	"github.com/tilt-dev/ctlptl/pkg/api"
)

type Machine interface {
	CPUs(ctx context.Context) (int, error)
	Apply(ctx context.Context, cluster *api.Cluster) (*api.Cluster, error)
}

type unknownMachine struct {
	product Product
}

func (m unknownMachine) CPUs(ctx context.Context) (int, error) {
	return 0, nil
}

func (m unknownMachine) Apply(ctx context.Context, c *api.Cluster) (*api.Cluster, error) {
	return nil, fmt.Errorf("cluster type %s not configurable", c.Product)
}

type dockerMachine struct {
	dockerClient *client.Client
}

func NewDockerMachine(ctx context.Context) (*dockerMachine, error) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	client.NegotiateAPIVersion(ctx)

	return &dockerMachine{
		dockerClient: client,
	}, nil
}

func (m dockerMachine) CPUs(ctx context.Context) (int, error) {
	info, err := m.dockerClient.Info(ctx)
	if err != nil {
		return 0, err
	}
	return info.NCPU, nil
}

func (m dockerMachine) Apply(ctx context.Context, c *api.Cluster) (*api.Cluster, error) {
	// TODO(nick): Implement this
	return nil, fmt.Errorf("cluster type %s not configurable", c.Product)
}

type minikubeMachine struct {
	name string
}

type minikubeSettings struct {
	CPUs int
}

func (m minikubeMachine) CPUs(ctx context.Context) (int, error) {
	homedir, err := homedir.Dir()
	if err != nil {
		return 0, err
	}
	configPath := filepath.Join(homedir, ".minikube", "profiles", m.name, "config.json")
	f, err := os.Open(configPath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	settings := minikubeSettings{}
	err = decoder.Decode(&settings)
	if err != nil {
		return 0, err
	}
	return settings.CPUs, nil
}

func (m minikubeMachine) Apply(ctx context.Context, c *api.Cluster) (*api.Cluster, error) {
	// TODO(nick): Implement this
	return nil, fmt.Errorf("cluster type minikube not configurable")
}
