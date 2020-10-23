package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/docker/docker/client"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/apimachinery/pkg/util/wait"
	klog "k8s.io/klog/v2"
)

type Machine interface {
	CPUs(ctx context.Context) (int, error)
	EnsureExists(ctx context.Context) error
	Restart(ctx context.Context, desired, existing *api.Cluster) error
}

type unknownMachine struct {
	product Product
}

func (m unknownMachine) EnsureExists(ctx context.Context) error {
	return fmt.Errorf("cluster type %s not configurable", m.product)
}

func (m unknownMachine) CPUs(ctx context.Context) (int, error) {
	return 0, nil
}

func (m unknownMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	return fmt.Errorf("cluster type %s not configurable", desired.Product)
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

func (m dockerMachine) EnsureExists(ctx context.Context) error {
	_, err := m.dockerClient.ServerVersion(ctx)
	if err == nil {
		return nil
	}

	klog.V(2).Infoln("No Docker daemon running. Attempting to start Docker.")
	if runtime.GOOS == "darwin" {
		_, err := os.Stat("/Applications/Docker.app")
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("Please install Docker for Desktop: https://www.docker.com/products/docker-desktop")
			}
			return err
		}

		cmd := exec.Command("open", "/Applications/Docker.app")
		err = cmd.Run()
		if err != nil {
			return errors.Wrap(err, "starting Docker")
		}

		err = wait.Poll(2*time.Second, 60*time.Second, func() (bool, error) {
			_, err := m.dockerClient.ServerVersion(ctx)
			isSuccess := err == nil
			return isSuccess, nil
		})
		if err != nil {
			return fmt.Errorf("timed out waiting for Docker to start")
		}
		klog.V(2).Infoln("Docker started successfully")
		return nil
	}

	if runtime.GOOS == "windows" {
		return fmt.Errorf("Please install Docker for Desktop: https://www.docker.com/products/docker-desktop")
	}
	return fmt.Errorf("Please install Docker for Linux: https://docs.docker.com/engine/install/")
}

func (m dockerMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	canChangeCPUs := runtime.GOOS == "darwin"
	if existing.Status.CPUs < desired.MinCPUs && !canChangeCPUs {
		return fmt.Errorf("Cannot automatically set minimum CPU to %d on this platform", desired.MinCPUs)
	}

	if runtime.GOOS == "darwin" {
		d4m, err := NewDockerForMacClient()
		if err != nil {
			return err
		}

		settings, err := d4m.settings(ctx)
		if err != nil {
			return err
		}

		k8sChanged, err := d4m.ensureK8sEnabled(settings)
		if err != nil {
			return err
		}

		cpuChanged, err := d4m.ensureMinCPU(settings, desired.MinCPUs)
		if err != nil {
			return err
		}

		if k8sChanged || cpuChanged {
			return d4m.writeSettings(ctx, settings)
		}
	}

	// TODO(nick): restart
	return nil
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

func (m minikubeMachine) EnsureExists(ctx context.Context) error {
	// TODO(nick): Implement this
	return fmt.Errorf("cluster type minikube not configurable")
}

func (m minikubeMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	// TODO(nick): Implement this
	return fmt.Errorf("cluster type minikube not configurable")
}
