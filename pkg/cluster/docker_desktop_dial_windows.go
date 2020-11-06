// +build windows

package cluster

import (
	"net"

	"gopkg.in/natefinch/npipe.v2"
)

func dockerDesktopSocketPath() (string, error) {
	return `\\.\pipe\dockerWebApiServer`, nil
}

func dialDockerDesktop(socketPath string) (net.Conn, error) {
	return npipe.Dial(socketPath)
}
