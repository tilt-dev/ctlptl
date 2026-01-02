package cluster

import (
	"context"
	"os"
	"regexp"

	"github.com/moby/moby/client"

	"github.com/tilt-dev/ctlptl/internal/dctr"
)

const (
	shortLen = 12
)

var (
	validShortID = regexp.MustCompile("^[a-f0-9]{12}$")
)

// IsShortID determines if id has the correct format and length for a short ID.
// It checks the IDs length and if it consists of valid characters for IDs (a-f0-9).
//
// Deprecated: this function is no longer used, and will be removed in the next release.
func isShortID(id string) bool {
	if len(id) != shortLen {
		return false
	}
	return validShortID.MatchString(id)
}

type detectInContainer interface {
	insideContainer(ctx context.Context) string
}

// InsideContainer checks the current host and docker client to see if we are
// running inside a container with a Docker-out-of-Docker-mounted socket. It
// checks if:
//
//   - The effective DOCKER_HOST is `/var/run/docker.sock`
//   - The hostname looks like a container "short id" and is a valid, running
//     container
//
// Returns a non-empty string representing the container ID if inside a container.
func insideContainer(ctx context.Context, dockerClient dctr.Client) string {
	// allows fake client to mock the result
	if detect, ok := dockerClient.(detectInContainer); ok {
		return detect.insideContainer(ctx)
	}

	if dockerClient.DaemonHost() != "unix:///var/run/docker.sock" {
		return ""
	}

	containerID, err := os.Hostname()
	if err != nil {
		return ""
	}

	if !isShortID(containerID) {
		return ""
	}

	container, err := dockerClient.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return ""
	}

	if !container.Container.State.Running {
		return ""
	}

	return containerID
}
