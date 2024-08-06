package dctr

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	registrytypes "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/docker/docker/registry"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/sdk/resource"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// Docker Container client.
type Client interface {
	DaemonHost() string
	ImagePull(ctx context.Context, image string, options image.PullOptions) (io.ReadCloser, error)

	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerRemove(ctx context.Context, id string, options container.RemoveOptions) error
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *specs.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error

	ServerVersion(ctx context.Context) (types.Version, error)
	Info(ctx context.Context) (system.Info, error)
	NetworkConnect(ctx context.Context, networkID, containerID string, config *network.EndpointSettings) error
	NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error
}

type CLI interface {
	Client() Client
	AuthInfo(ctx context.Context, repoInfo *registry.RepositoryInfo, cmdName string) (string, types.RequestPrivilegeFunc, error)
}

type realCLI struct {
	cli *command.DockerCli
}

func (c *realCLI) Client() Client {
	return c.cli.Client()
}

func (c *realCLI) AuthInfo(ctx context.Context, repoInfo *registry.RepositoryInfo, cmdName string) (string, types.RequestPrivilegeFunc, error) {
	authConfig := command.ResolveAuthConfig(c.cli.ConfigFile(), repoInfo.Index)
	requestPrivilege := command.RegistryAuthenticationPrivilegedFunc(c.cli, repoInfo.Index, cmdName)

	auth, err := registrytypes.EncodeAuthConfig(authConfig)
	if err != nil {
		return "", nil, errors.Wrap(err, "authInfo#EncodeAuthToBase64")
	}
	return auth, requestPrivilege, nil
}

func NewCLI(streams genericclioptions.IOStreams) (CLI, error) {
	dockerCli, err := command.NewDockerCli(
		command.WithOutputStream(streams.Out),
		command.WithErrorStream(streams.ErrOut),
		command.WithResource(resource.Empty()))
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
	co, err := c.ContainerInspect(ctx, name)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil
		}
		return err
	}
	if co.ContainerJSONBase == nil {
		return nil
	}

	return c.ContainerRemove(ctx, co.ID, container.RemoveOptions{
		Force: true,
	})
}

// A simplified run-container-and-detach helper for background support containers (like socat and the registry).
func Run(ctx context.Context, cli CLI, name string, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig) error {
	c := cli.Client()

	ctr, err := c.ContainerInspect(ctx, name)
	if err == nil && (ctr.ContainerJSONBase != nil && ctr.State.Running) {
		// The service is already running!
		return nil
	} else if err == nil {
		// The service exists, but is not running
		err := c.ContainerRemove(ctx, name, container.RemoveOptions{Force: true})
		if err != nil {
			return fmt.Errorf("creating %s: %v", name, err)
		}
	} else if !client.IsErrNotFound(err) {
		return fmt.Errorf("inspecting %s: %v", name, err)
	}

	resp, err := c.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, name)
	if err != nil {
		if !client.IsErrNotFound(err) {
			return fmt.Errorf("creating %s: %v", name, err)
		}

		err := pull(ctx, cli, config.Image)
		if err != nil {
			return fmt.Errorf("pulling image %s: %v", config.Image, err)
		}

		resp, err = c.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, name)
		if err != nil {
			return fmt.Errorf("creating %s: %v", name, err)
		}
	}

	id := resp.ID
	err = c.ContainerStart(ctx, id, container.StartOptions{})
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

	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return fmt.Errorf("could not parse registry for %q: %v", img, err)
	}

	encodedAuth, requestPrivilege, err := cli.AuthInfo(ctx, repoInfo, "pull")
	if err != nil {
		return fmt.Errorf("could not authenticate: %v", err)
	}

	resp, err := c.ImagePull(ctx, img, image.PullOptions{
		RegistryAuth:  encodedAuth,
		PrivilegeFunc: requestPrivilege,
	})
	if err != nil {
		return fmt.Errorf("pulling image %s: %v", img, err)
	}
	defer resp.Close()

	_, _ = io.Copy(io.Discard, resp)
	return nil
}
