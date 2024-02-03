/*
Copyright © 2020-2022 The k3d Author(s)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package k3dv1alpha5

import (
	"time"
)

// TypeMeta partially copies apimachinery/pkg/apis/meta/v1.TypeMeta
// No need for a direct dependence; the fields are stable.
type TypeMeta struct {
	Kind       string `yaml:"kind,omitempty"`
	APIVersion string `yaml:"apiVersion,omitempty"`
}

type ObjectMeta struct {
	Name string `mapstructure:"name,omitempty" yaml:"name,omitempty"`
}

type VolumeWithNodeFilters struct {
	Volume      string   `mapstructure:"volume" yaml:"volume,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty"`
}

type PortWithNodeFilters struct {
	Port        string   `mapstructure:"port" yaml:"port,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty"`
}

type LabelWithNodeFilters struct {
	Label       string   `mapstructure:"label" yaml:"label,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty"`
}

type EnvVarWithNodeFilters struct {
	EnvVar      string   `mapstructure:"envVar" yaml:"envVar,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty"`
}

type K3sArgWithNodeFilters struct {
	Arg         string   `mapstructure:"arg" yaml:"arg,omitempty"`
	NodeFilters []string `mapstructure:"nodeFilters" yaml:"nodeFilters,omitempty"`
}

type SimpleConfigRegistryCreateConfig struct {
	Name     string   `mapstructure:"name" yaml:"name,omitempty"`
	Host     string   `mapstructure:"host" yaml:"host,omitempty"`
	HostPort string   `mapstructure:"hostPort" yaml:"hostPort,omitempty"`
	Image    string   `mapstructure:"image" yaml:"image,omitempty"`
	Volumes  []string `mapstructure:"volumes" yaml:"volumes,omitempty"`
}

// SimpleConfigOptionsKubeconfig describes the set of options referring to the kubeconfig during cluster creation.
type SimpleConfigOptionsKubeconfig struct {
	UpdateDefaultKubeconfig bool `mapstructure:"updateDefaultKubeconfig" yaml:"updateDefaultKubeconfig,omitempty"` // default: true
	SwitchCurrentContext    bool `mapstructure:"switchCurrentContext" yaml:"switchCurrentContext,omitempty"`       //nolint:lll    // default: true
}

type SimpleConfigOptions struct {
	K3dOptions        SimpleConfigOptionsK3d        `mapstructure:"k3d" yaml:"k3d"`
	K3sOptions        SimpleConfigOptionsK3s        `mapstructure:"k3s" yaml:"k3s"`
	KubeconfigOptions SimpleConfigOptionsKubeconfig `mapstructure:"kubeconfig" yaml:"kubeconfig"`
	Runtime           SimpleConfigOptionsRuntime    `mapstructure:"runtime" yaml:"runtime"`
}

type SimpleConfigOptionsRuntime struct {
	GPURequest    string                 `mapstructure:"gpuRequest" yaml:"gpuRequest,omitempty"`
	ServersMemory string                 `mapstructure:"serversMemory" yaml:"serversMemory,omitempty"`
	AgentsMemory  string                 `mapstructure:"agentsMemory" yaml:"agentsMemory,omitempty"`
	HostPidMode   bool                   `mapstructure:"hostPidMode" yyaml:"hostPidMode,omitempty"`
	Labels        []LabelWithNodeFilters `mapstructure:"labels" yaml:"labels,omitempty"`
	Ulimits       []Ulimit               `mapstructure:"ulimits" yaml:"ulimits,omitempty"`
}

type Ulimit struct {
	Name string `mapstructure:"name" yaml:"name"`
	Soft int64  `mapstructure:"soft" yaml:"soft"`
	Hard int64  `mapstructure:"hard" yaml:"hard"`
}

type SimpleConfigOptionsK3d struct {
	Wait                bool                               `mapstructure:"wait" yaml:"wait"`
	Timeout             time.Duration                      `mapstructure:"timeout" yaml:"timeout,omitempty"`
	DisableLoadbalancer bool                               `mapstructure:"disableLoadbalancer" yaml:"disableLoadbalancer"`
	DisableImageVolume  bool                               `mapstructure:"disableImageVolume" yaml:"disableImageVolume"`
	NoRollback          bool                               `mapstructure:"disableRollback" yaml:"disableRollback"`
	Loadbalancer        SimpleConfigOptionsK3dLoadbalancer `mapstructure:"loadbalancer" yaml:"loadbalancer,omitempty"`
}

type SimpleConfigOptionsK3dLoadbalancer struct {
	ConfigOverrides []string `mapstructure:"configOverrides" yaml:"configOverrides,omitempty"`
}

type SimpleConfigOptionsK3s struct {
	ExtraArgs  []K3sArgWithNodeFilters `mapstructure:"extraArgs" yaml:"extraArgs,omitempty"`
	NodeLabels []LabelWithNodeFilters  `mapstructure:"nodeLabels" yaml:"nodeLabels,omitempty"`
}

type SimpleConfigRegistries struct {
	Use    []string                          `mapstructure:"use" yaml:"use,omitempty"`
	Create *SimpleConfigRegistryCreateConfig `mapstructure:"create" yaml:"create,omitempty"`
	Config string                            `mapstructure:"config" yaml:"config,omitempty"` // registries.yaml (k3s config for containerd registry override)
}

type SimpleConfigHostAlias struct {
	IP        string   `mapstructure:"ip" yaml:"ip" json:"ip"`
	Hostnames []string `mapstructure:"hostnames" yaml:"hostnames" json:"hostnames"`
}

// SimpleConfig describes the toplevel k3d configuration file.
type SimpleConfig struct {
	TypeMeta     `mapstructure:",squash" yaml:",inline"`
	ObjectMeta   `mapstructure:"metadata" yaml:"metadata,omitempty"`
	Servers      int                     `mapstructure:"servers" yaml:"servers,omitempty"` //nolint:lll    // default 1
	Agents       int                     `mapstructure:"agents" yaml:"agents,omitempty"`   //nolint:lll    // default 0
	ExposeAPI    SimpleExposureOpts      `mapstructure:"kubeAPI" yaml:"kubeAPI,omitempty"`
	Image        string                  `mapstructure:"image" yaml:"image,omitempty"`
	Network      string                  `mapstructure:"network" yaml:"network,omitempty"`
	Subnet       string                  `mapstructure:"subnet" yaml:"subnet,omitempty"`
	ClusterToken string                  `mapstructure:"token" yaml:"clusterToken,omitempty"` // default: auto-generated
	Volumes      []VolumeWithNodeFilters `mapstructure:"volumes" yaml:"volumes,omitempty"`
	Ports        []PortWithNodeFilters   `mapstructure:"ports" yaml:"ports,omitempty"`
	Options      SimpleConfigOptions     `mapstructure:"options" yaml:"options,omitempty"`
	Env          []EnvVarWithNodeFilters `mapstructure:"env" yaml:"env,omitempty"`
	Registries   SimpleConfigRegistries  `mapstructure:"registries" yaml:"registries,omitempty"`
	HostAliases  []SimpleConfigHostAlias `mapstructure:"hostAliases" yaml:"hostAliases,omitempty"`
}

// SimpleExposureOpts provides a simplified syntax compared to the original k3d.ExposureOpts
type SimpleExposureOpts struct {
	Host     string `mapstructure:"host" yaml:"host,omitempty"`
	HostIP   string `mapstructure:"hostIP" yaml:"hostIP,omitempty"`
	HostPort string `mapstructure:"hostPort" yaml:"hostPort,omitempty"`
}
