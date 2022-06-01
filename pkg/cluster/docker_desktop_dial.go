//go:build !windows
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

	switch runtime.GOOS {
	case "darwin":
		return []string{
			// Older versions of docker desktop use this socket.
			filepath.Join(homedir, "Library/Containers/com.docker.docker/Data/gui-api.sock"),

			// Newer versions of docker desktop use this socket.
			filepath.Join(homedir, "Library/Containers/com.docker.docker/Data/backend.native.sock"),
		}, nil
	case "linux":
		return []string{
			// Docker Desktop for Linux
			filepath.Join(homedir, ".docker/desktop/backend.native.sock"),
		}, nil
	}
	return nil, fmt.Errorf("Cannot find docker-desktop socket paths on %s", runtime.GOOS)
}

func dialDockerDesktop(socketPath string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("Cannot dial docker-desktop on %s", runtime.GOOS)
	}

	return net.Dial("unix", socketPath)
}
