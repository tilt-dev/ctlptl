// Manage socat network routers for remote docker instances.
package socat

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const serviceName = "ctlptl-portforward-service"

type ContainerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerRemove(ctx context.Context, id string, options types.ContainerRemoveOptions) error
}

type Controller struct {
	client ContainerClient
}

func NewController(client ContainerClient) *Controller {
	return &Controller{client: client}
}

func DefaultController(ctx context.Context) (*Controller, error) {
	client, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}

	client.NegotiateAPIVersion(ctx)
	return NewController(client), nil
}

// Connect a port on the local machine to a port on a remote docker machine.
func (c *Controller) ConnectRemoteDockerPort(ctx context.Context, port int) error {
	err := c.StartRemotePortforwarder(ctx)
	if err != nil {
		return err
	}
	return c.StartLocalPortforwarder(ctx, port)
}

// Create a port-forwarding server on the same machine that's running
// Docker. This server accepts connections and routes them to localhost ports
// on the same machine.
func (c *Controller) StartRemotePortforwarder(ctx context.Context) error {
	container, err := c.client.ContainerInspect(ctx, serviceName)
	if err == nil && container.State.Running {
		// The service is already running!
		return nil
	} else if err == nil {
		// The service exists, but is not running
		err := c.client.ContainerRemove(ctx, serviceName, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			return fmt.Errorf("creating remote portforwarder: %v", err)
		}
	} else if !client.IsErrNotFound(err) {
		return fmt.Errorf("inspecting remote portforwarder: %v", err)
	}

	cmd := exec.Command("docker", "run", "-d", "-it",
		"--name", serviceName, "--net=host", "--restart=always",
		"--entrypoint", "/bin/sh", "alpine/socat", "-c", "while true; do sleep 1000; done")
	return cmd.Run()
}

// Create a port-forwarding server on the local machine, forwarding connections
// to the same port on the remote Docker server.
func (c *Controller) StartLocalPortforwarder(ctx context.Context, port int) error {
	cmd := exec.Command("socat", fmt.Sprintf("TCP-LISTEN:%d,reuseaddr,fork", port),
		fmt.Sprintf("EXEC:'docker exec -i %s socat STDIO TCP:localhost:%d'", serviceName, port))
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("creating local portforwarder: %v", err)
	}

	for i := 0; i < 100; i++ {
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for local portforwarder")
}
