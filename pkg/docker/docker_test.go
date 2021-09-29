package docker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type dockerHostTestCase struct {
	host     string
	expected bool
}

func TestIsLocalDockerHost(t *testing.T) {
	cases := []dockerHostTestCase{
		dockerHostTestCase{"", true},
		dockerHostTestCase{"tcp://localhost:2375", true},
		dockerHostTestCase{"tcp://127.0.0.1:2375", true},
		dockerHostTestCase{"npipe:////./pipe/docker_engine", true},
		dockerHostTestCase{"unix:///var/run/docker.sock", true},
		dockerHostTestCase{"tcp://cluster:2375", false},
		dockerHostTestCase{"http://cluster:2375", false},
	}
	for i, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%s-%d", t.Name(), i), func(t *testing.T) {
			assert.Equal(t, c.expected, IsLocalHost(c.host))
		})
	}
}
