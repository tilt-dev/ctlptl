package registry

import (
	"github.com/tilt-dev/ctlptl/pkg/api"
	"k8s.io/apimachinery/pkg/fields"
)

type ListOptions struct {
	FieldSelector string
}

type registryFields api.Registry

func (cf *registryFields) Has(field string) bool {
	return field == "name"
}

func (cf *registryFields) Get(field string) string {
	if field == "name" {
		return (*api.Registry)(cf).Name
	}
	return ""
}

var _ fields.Fields = &registryFields{}
