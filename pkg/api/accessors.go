package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Cluster) GetObjectMeta() metav1.Object {
	return &metav1.ObjectMeta{
		Name: c.Name,
	}
}

func (r *Registry) GetObjectMeta() metav1.Object {
	return &metav1.ObjectMeta{
		Name: r.Name,
	}
}

var _ metav1.ObjectMetaAccessor = &Cluster{}
var _ metav1.ObjectMetaAccessor = &Registry{}
