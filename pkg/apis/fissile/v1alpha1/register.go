// NOTE: Boilerplate only.  Ignore this file.

// Package v1alpha1 contains API Schema definitions for the fissile v1alpha1 API group
// +k8s:deepcopy-gen=package,register
// +groupName=fissile.cloudfoundry.org
package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
)

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: "fissile.cloudfoundry.org", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = &scheme.Builder{GroupVersion: SchemeGroupVersion}
)

// AddToScheme adds our Resource to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	addToSchemes := runtime.SchemeBuilder{SchemeBuilder.AddToScheme}
	return addToSchemes.AddToScheme(s)
}
