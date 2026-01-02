package dctr

import (
	"context"
	"fmt"
	"io"

	"github.com/containerd/errdefs"
	"github.com/distribution/reference"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Docker Container client.
type Client interface {
	DaemonHost() string
	ImagePull(ctx context.Context, image string, options client.ImagePullOptions) (client.ImagePullResponse, error)

	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerInspect(ctx context.Context, containerID string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerRemove(ctx context.Context, id string, options client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) (client.ContainerStartResult, error)

	ServerVersion(ctx context.Context, options client.ServerVersionOptions) (client.ServerVersionResult, error)
	Info(ctx context.Context, options client.InfoOptions) (client.SystemInfoResult, error)
	NetworkConnect(ctx context.Context, networkID string, options client.NetworkConnectOptions) (client.NetworkConnectResult, error)
	NetworkDisconnect(ctx context.Context, networkID string, options client.NetworkDisconnectOptions) (client.NetworkDisconnectResult, error)
}

type CLI interface {
	Client() Client
	AuthInfo(ctx context.Context, ref reference.Reference, cmdName string) (string, error)
}

type realCLI struct {
	cli *command.DockerCli
}

func (c *realCLI) Client() Client {
	return c.cli.Client()
}

func (c *realCLI) AuthInfo(ctx context.Context, ref reference.Reference, cmdName string) (string, error) {
	return command.RetrieveAuthTokenFromImage(c.cli.ConfigFile(), ref.String())
}

func NewCLI(streams genericclioptions.IOStreams) (CLI, error) {
	dockerCli, err := command.NewDockerCli(
		command.WithOutputStream(streams.Out),
		command.WithErrorStream(streams.ErrOut))
	if err != nil {
		return nil, fmt.Errorf("failed to create new docker API: %v", err)
	}

	opts := cliflags.NewClientOptions()
	flagSet := pflag.NewFlagSet("docker", pflag.ContinueOnError)
	opts.InstallFlags(flagSet)
	opts.SetDefaultOptions(flagSet)
	err = dockerCli.Initialize(opts)
	if err != nil {
		return nil, fmt.Errorf("initializing docker client: %v", err)
	}

	// A hack to see if initialization failed.
	// https://github.com/docker/cli/issues/4489
	endpoint := dockerCli.DockerEndpoint()
	if endpoint.Host == "" {
		return nil, fmt.Errorf("initializing docker client: no valid endpoint")
	}
	return &realCLI{cli: dockerCli}, nil
}

func NewAPIClient(streams genericclioptions.IOStreams) (Client, error) {
	cli, err := NewCLI(streams)
	if err != nil {
		return nil, err
	}
	return cli.Client(), nil
}

// A simplified remove-container-if-necessary helper.
func RemoveIfNecessary(ctx context.Context, c Client, name string) error {
	co, err := c.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return err
	}

	_, err = c.ContainerRemove(ctx, co.Container.ID, client.ContainerRemoveOptions{
		Force: true,
	})
	return err
}

// A simplified run-container-and-detach helper for background support containers (like socat and the registry).
func Run(ctx context.Context, cli CLI, name string, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig) error {
	c := cli.Client()

	ctr, err := c.ContainerInspect(ctx, name, client.ContainerInspectOptions{})
	if err == nil && ctr.Container.State.Running {
		// The service is already running!
		return nil
	} else if err == nil {
		// The service exists, but is not running
		_, err := c.ContainerRemove(ctx, name, client.ContainerRemoveOptions{Force: true})
		if err != nil {
			return fmt.Errorf("creating %s: %v", name, err)
		}
	} else if !errdefs.IsNotFound(err) {
		return fmt.Errorf("inspecting %s: %v", name, err)
	}

	resp, err := c.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Name:             name,
	})
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return fmt.Errorf("creating %s: %v", name, err)
		}

		err := pull(ctx, cli, config.Image)
		if err != nil {
			return fmt.Errorf("pulling image %s: %v", config.Image, err)
		}

		resp, err = c.ContainerCreate(ctx, client.ContainerCreateOptions{
			Config:           config,
			HostConfig:       hostConfig,
			NetworkingConfig: networkingConfig,
			Name:             name,
		})
		if err != nil {
			return fmt.Errorf("creating %s: %v", name, err)
		}
	}

	id := resp.ID
	_, err = c.ContainerStart(ctx, id, client.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("starting %s: %v", name, err)
	}
	return nil
}

func pull(ctx context.Context, cli CLI, img string) error {
	c := cli.Client()

	ref, err := reference.ParseNormalizedNamed(img)
	if err != nil {
		return fmt.Errorf("could not parse image %q: %v", img, err)
	}

	encodedAuth, err := cli.AuthInfo(ctx, ref, "pull")
	if err != nil {
		return fmt.Errorf("could not authenticate: %v", err)
	}

	resp, err := c.ImagePull(ctx, img, client.ImagePullOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return fmt.Errorf("pulling image %s: %v", img, err)
	}
	defer func() {
		_ = resp.Close()
	}()

	_, _ = io.Copy(io.Discard, resp)
	return nil
}
