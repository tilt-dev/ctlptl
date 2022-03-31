package docker

import (
	"net/http"
	"os"
	"strings"

	"github.com/docker/cli/cli/connhelper"
	"github.com/docker/docker/client"
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
		(strings.HasPrefix(dockerHost, "unix:") &&
			// https://docs.docker.com/desktop/faqs/#how-do-i-connect-to-the-remote-docker-engine-api
			strings.Contains(dockerHost, "/var/run/docker.sock"))
}

// ClientOpts returns an appropiate slice of client.Opt values for connecting to a Docker client.
// It can support using SSH connections via the Docker CLI's connection helpers if the DOCKER_HOST
// environment variable is an SSH url, otherwise it will return client.FromEnv for a standard
// connection. This function returns an error if DOCKER_HOST is an invalid URL.
func ClientOpts() ([]client.Opt, error) {
	opts := []client.Opt{client.FromEnv}
	connHelperOpts, err := connectionHelperOpts()
	if err != nil {
		return nil, err
	}
	if connHelperOpts != nil {
		opts = append(opts, connHelperOpts...)
	}
	return opts, nil
}

// connectionHelperOpts uses the Docker CLI's connection helpers to check if the DOCKER_HOST
// setting needs some special connection functionality, namely SSH. If it does, it will return
// a list of appropriate client options. If not, it will return nil.
func connectionHelperOpts() ([]client.Opt, error) {
	dockerHost := GetHostEnv()
	if dockerHost != "" {
		helper, err := connhelper.GetConnectionHelper(dockerHost)
		if err != nil {
			return nil, err
		}

		if helper != nil {
			// Create an HTTP client with a transport that reads from the connection helper's
			// custom stream without TLS.
			httpClient := &http.Client{
				Transport: &http.Transport{
					DialContext: helper.Dialer,
				},
			}
			return []client.Opt{
				client.WithHTTPClient(httpClient),
				client.WithHost(helper.Host),
				client.WithDialContext(helper.Dialer),
			}, nil
		}

	}
	return nil, nil
}
