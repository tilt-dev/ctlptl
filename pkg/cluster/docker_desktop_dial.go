// +build !windows

package cluster

import (
	"fmt"
	"net"
	"path/filepath"
	"runtime"

	"github.com/mitchellh/go-homedir"
)

func dockerDesktopSocketPaths() ([]string, error) {
	homedir, err := homedir.Dir()
	if err != nil {
		return nil, err
	}

	return []string{
		// Older versions of docker desktop use this socket.
		filepath.Join(homedir, "Library/Containers/com.docker.docker/Data/gui-api.sock"),

		// Newer versions of docker desktop use this socket.
		filepath.Join(homedir, "Library/Containers/com.docker.docker/Data/backend.sock"),
	}, nil
}

func dialDockerDesktop(socketPath string) (net.Conn, error) {
	if runtime.GOOS != "darwin" {
		return nil, fmt.Errorf("Cannot dial docker-desktop on %s", runtime.GOOS)
	}

	return net.Dial("unix", socketPath)
}
