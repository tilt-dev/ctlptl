package api

import (
	"github.com/tilt-dev/localregistry-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TypeMeta partially copies apimachinery/pkg/apis/meta/v1.TypeMeta
// No need for a direct dependence; the fields are stable.
type TypeMeta struct {
	Kind       string `json:"kind,omitempty" yaml:"kind,omitempty"`
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
}

// Cluster contains cluster configuration.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Cluster struct {
	TypeMeta `yaml:",inline"`

	// The cluster name. Pulled from .kube/config.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// The name of the tool used to create this cluster.
	Product string `json:"product,omitempty" yaml:"product,omitempty"`

	// Make sure that the cluster has access to at least this many
	// CPUs. This is mostly helpful for ensuring that your Docker Desktop
	// VM has enough CPU. If ctlptl can't guarantee this many
	// CPU, it will return an error.
	MinCPUs int `json:"minCPUs,omitempty" yaml:"minCPUs,omitempty"`

	// The name of a registry.
	//
	// If the registry doesn't exist, ctlptl will create one with this name.
	//
	// The registry can be configured by creating a `kind: Registry` config file.
	//
	// Not supported on all cluster products.
	Registry string `json:"registry,omitempty" yaml:"registry,omitempty"`

	// Most recently observed status of the cluster.
	// Populated by the system.
	// Read-only.
	Status ClusterStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type ClusterStatus struct {
	// When the cluster was first created.
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty" yaml:"creationTimestamp,omitempty"`

	// Local registry status documented on the cluster itself.
	LocalRegistryHosting *localregistry.LocalRegistryHostingV1 `json:"localRegistryHosting,omitempty" yaml:"localRegistryHosting,omitempty"`

	// The number of CPU. Only applicable to local clusters.
	CPUs int `json:"cpus,omitempty" yaml:"cpus,omitempty"`
}

// ClusterList is a list of Clusters.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	TypeMeta `json:",inline"`

	// List of clusters.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md
	Items []Cluster `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// Cluster contains registry configuration.
//
// Currently designed for local registries on the host machine, but
// may eventually expand to support remote registries.
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Registry struct {
	TypeMeta `yaml:",inline"`

	// The registry name. Get/set from the Docker container name.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// The desired host port. Set to 0 to choose a random port.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// Most recently observed status of the registry.
	// Populated by the system.
	// Read-only.
	Status RegistryStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

type RegistryStatus struct {
	// When the registry was first created.
	CreationTimestamp metav1.Time `json:"creationTimestamp,omitempty" yaml:"creationTimestamp,omitempty"`

	// The IPv4 address for the bridge network.
	IPAddress string `json:"ipAddress,omitempty" yaml:"ipAddress,omitempty"`

	// The public port that the registry is listening on on the host machine.
	HostPort int `json:"hostPort,omitempty" yaml:"hostPort,omitempty"`

	// The private port that the registry is listening on inside the registry network.
	//
	// We try to make this not configurable, because there's no real reason not
	// to use the default registry port 5000.
	ContainerPort int `json:"containerPort,omitempty" yaml:containerPort,omitempty"`

	// Networks that the registry container is connected to.
	Networks []string `json:"networks,omitempty" yaml:"networks,omitempty"`

	// The ID of the container in Docker.
	ContainerID string `json:"containerId,omitempty" yaml:"containerId,omitempty"`
}

// RegistryList is a list of Registrys.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RegistryList struct {
	TypeMeta `json:",inline"`

	// List of registrys.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md
	Items []Registry `json:"items" protobuf:"bytes,2,rep,name=items"`
}
