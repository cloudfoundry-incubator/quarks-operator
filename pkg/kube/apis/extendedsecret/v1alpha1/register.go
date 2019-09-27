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
	// ExtendedSecretResourceKind is the kind name of ExtendedSecret
	ExtendedSecretResourceKind = "ExtendedSecret"
	// ExtendedSecretResourcePlural is the plural name of ExtendedSecret
	ExtendedSecretResourcePlural = "extendedsecrets"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is used for schema registrations in the controller package
	// and also in the generated kube code
	AddToScheme = schemeBuilder.AddToScheme

	// ExtendedSecretResourceShortNames is the short names of ExtendedSecret
	ExtendedSecretResourceShortNames = []string{"esec", "esecs"}

	// ExtendedSecretResourceName is the resource name of ExtendedSecret
	ExtendedSecretResourceName = fmt.Sprintf("%s.%s", ExtendedSecretResourcePlural, apis.GroupName)

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
		&ExtendedSecret{},
		&ExtendedSecretList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
