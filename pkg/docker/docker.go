package docker

import (
	"os"
	"strings"
)

func GetHostEnv() string {
	return os.Getenv("DOCKER_HOST")
}

func IsLocalHost(dockerHost string) bool {
	return dockerHost == "" ||

		// Check all the "standard" docker localhosts.
		// https://github.com/docker/cli/blob/a32cd16160f1b41c1c4ae7bee4dac929d1484e59/opts/hosts.go#L22
		strings.HasPrefix(dockerHost, "tcp://localhost:") ||
		strings.HasPrefix(dockerHost, "tcp://127.0.0.1:") ||

		// https://github.com/moby/moby/blob/master/client/client_windows.go#L4
		strings.HasPrefix(dockerHost, "npipe:") ||

		// https://github.com/moby/moby/blob/master/client/client_unix.go#L6
		strings.HasPrefix(dockerHost, "unix:")
}
