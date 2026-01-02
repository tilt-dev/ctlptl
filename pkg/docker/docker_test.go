package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type dockerHostTestCase struct {
	host          string
	localDaemon   bool
	dockerDesktop bool
}

func TestIsLocalDockerHost(t *testing.T) {
	cases := []dockerHostTestCase{
		dockerHostTestCase{"", true, true},
		dockerHostTestCase{"tcp://localhost:2375", true, true},
		dockerHostTestCase{"tcp://127.0.0.1:2375", true, true},
		dockerHostTestCase{"npipe:////./pipe/docker_engine", true, true},
		dockerHostTestCase{"unix:///var/run/docker.sock", true, true},
		dockerHostTestCase{"tcp://cluster:2375", false, false},
		dockerHostTestCase{"http://cluster:2375", false, false},
		dockerHostTestCase{"unix:///Users/USER/.colima/docker.sock", true, false},
		dockerHostTestCase{"unix:///Users/USER/.docker/desktop/docker.sock", true, true},
		dockerHostTestCase{"unix:///Users/USER/.docker/run/docker.sock", true, true},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%s-%d", t.Name(), i), func(t *testing.T) {
			assert.Equal(t, c.localDaemon, IsLocalHost(c.host))
			assert.Equal(t, c.dockerDesktop, IsLocalDockerEngineHost(c.host))
		})
	}
}

type dockerDesktopTestCase struct {
	host     string
	os       string
	expected bool
}

func TestIsLocalDockerDesktop(t *testing.T) {
	cases := []dockerDesktopTestCase{
		dockerDesktopTestCase{"", "linux", false},
		dockerDesktopTestCase{"tcp://localhost:2375", "linux", false},
		dockerDesktopTestCase{"tcp://127.0.0.1:2375", "linux", false},
		dockerDesktopTestCase{"npipe:////./pipe/docker_engine", "windows", true},
		dockerDesktopTestCase{"unix:///var/run/docker.sock", "darwin", true},
		dockerDesktopTestCase{"unix:///var/run/docker.sock", "linux", false},
		dockerDesktopTestCase{"tcp://cluster:2375", "linux", false},
		dockerDesktopTestCase{"http://cluster:2375", "linux", false},
		dockerDesktopTestCase{"unix:///Users/USER/.colima/docker.sock", "linux", false},
		dockerDesktopTestCase{"unix:///Users/USER/.docker/desktop/docker.sock", "linux", true},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("%s-%d", t.Name(), i), func(t *testing.T) {
			assert.Equal(t, c.expected, IsLocalDockerDesktop(c.host, c.os))
		})
	}
}
