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
