package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/ctlptl/pkg/api"
	"github.com/tilt-dev/ctlptl/pkg/docker"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/wait"
	klog "k8s.io/klog/v2"
)

type Machine interface {
	CPUs(ctx context.Context) (int, error)
	EnsureExists(ctx context.Context) error
	Restart(ctx context.Context, desired, existing *api.Cluster) error
}

type unknownMachine struct {
	product clusterid.Product
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

type sleeper func(dur time.Duration)

type d4mClient interface {
	writeSettings(ctx context.Context, settings map[string]interface{}) error
	settings(ctx context.Context) (map[string]interface{}, error)
	ResetCluster(tx context.Context) error
	setK8sEnabled(settings map[string]interface{}, desired bool) (bool, error)
	ensureMinCPU(settings map[string]interface{}, desired int) (bool, error)
	Open(ctx context.Context) error
}

type dockerMachine struct {
	dockerClient dockerClient
	errOut       io.Writer
	sleep        sleeper
	d4m          d4mClient
	os           string
}

func NewDockerMachine(ctx context.Context, client dockerClient, errOut io.Writer) (*dockerMachine, error) {
	d4m, err := NewDockerDesktopClient()
	if err != nil {
		return nil, err
	}

	return &dockerMachine{
		dockerClient: client,
		errOut:       errOut,
		sleep:        time.Sleep,
		d4m:          d4m,
		os:           runtime.GOOS,
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

	if !m.dockerClient.IsLocalDockerEngine() {
		return fmt.Errorf("Detected remote DOCKER_HOST, but no Docker running. Host: %q. Error: %v",
			docker.GetHostEnv(), err)
	}

	klog.V(2).Infoln("No Docker daemon running. Attempting to start Docker.")
	if m.os == "darwin" || m.os == "windows" {
		err := m.d4m.Open(ctx)
		if err != nil {
			return err
		}

		dur := 60 * time.Second
		_, _ = fmt.Fprintf(m.errOut, "Waiting %s for Docker Desktop to boot...\n", duration.ShortHumanDuration(dur))
		err = wait.Poll(time.Second, dur, func() (bool, error) {
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

	return fmt.Errorf("Please install Docker for Linux: https://docs.docker.com/engine/install/")
}

func (m dockerMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	canChangeCPUs := false
	isLocalDockerDesktop := false
	if m.dockerClient.IsLocalDockerEngine() && (m.os == "darwin" || m.os == "windows") {
		canChangeCPUs = true // DockerForMac and DockerForWindows can change the CPU on the VM
		isLocalDockerDesktop = true
	} else if clusterid.Product(desired.Product) == clusterid.ProductMinikube {
		// Minikube can change the CPU on the VM or on the container itself
		canChangeCPUs = true
	}

	if existing.Status.CPUs < desired.MinCPUs && !canChangeCPUs {
		return fmt.Errorf("Cannot automatically set minimum CPU to %d on this platform", desired.MinCPUs)
	}

	if isLocalDockerDesktop {
		settings, err := m.d4m.settings(ctx)
		if err != nil {
			return err
		}

		k8sChanged := false
		if desired.Product == string(clusterid.ProductDockerDesktop) {
			k8sChanged, err = m.d4m.setK8sEnabled(settings, true)
			if err != nil {
				return err
			}
		}

		cpuChanged, err := m.d4m.ensureMinCPU(settings, desired.MinCPUs)
		if err != nil {
			return err
		}

		if k8sChanged || cpuChanged {
			err := m.d4m.writeSettings(ctx, settings)
			if err != nil {
				return err
			}

			dur := 120 * time.Second
			_, _ = fmt.Fprintf(m.errOut,
				"Applied new Docker Desktop settings. Waiting %s for Docker Desktop to restart...\n",
				duration.ShortHumanDuration(dur))

			// Sleep for short time to ensure the write takes effect.
			m.sleep(2 * time.Second)

			err = wait.Poll(time.Second, dur, func() (bool, error) {
				_, err := m.dockerClient.ServerVersion(ctx)
				isSuccess := err == nil
				return isSuccess, nil
			})
			if err != nil {
				return errors.Wrap(err, "Docker Desktop restart timeout")
			}
		}
	}

	return nil
}

// Currently, out Minikube admin only supports Minikube on Docker,
// so we delegate to the dockerMachine driver.
type minikubeMachine struct {
	dm   *dockerMachine
	name string
}

func newMinikubeMachine(name string, dm *dockerMachine) *minikubeMachine {
	return &minikubeMachine{
		name: name,
		dm:   dm,
	}
}

type minikubeSettings struct {
	CPUs int
}

func (m *minikubeMachine) CPUs(ctx context.Context) (int, error) {
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

func (m *minikubeMachine) EnsureExists(ctx context.Context) error {
	return m.dm.EnsureExists(ctx)
}

func (m *minikubeMachine) Restart(ctx context.Context, desired, existing *api.Cluster) error {
	return m.dm.Restart(ctx, desired, existing)
}
