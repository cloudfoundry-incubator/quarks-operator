package v1alpha1

import (
	"fmt"

	apis "code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// This file looks almost the same for all controllers
// Modify the addKnownTypes function, then run `make generate`

const (
	// QuarksSecretResourceKind is the kind name of QuarksSecret
	QuarksSecretResourceKind = "QuarksSecret"
	// QuarksSecretResourcePlural is the plural name of QuarksSecret
	QuarksSecretResourcePlural = "quarkssecrets"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is used for schema registrations in the controller package
	// and also in the generated kube code
	AddToScheme = schemeBuilder.AddToScheme

	// QuarksSecretResourceShortNames is the short names of QuarksSecret
	QuarksSecretResourceShortNames = []string{"qsec", "qsecs"}

	// QuarksSecretResourceName is the resource name of QuarksSecret
	QuarksSecretResourceName = fmt.Sprintf("%s.%s", QuarksSecretResourcePlural, apis.GroupName)

	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: apis.GroupName, Version: "v1alpha1"}
)

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return SchemeGroupVersion.WithKind(kind).GroupKind()
}

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&QuarksSecret{},
		&QuarksSecretList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
