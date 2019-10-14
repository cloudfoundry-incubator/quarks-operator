package v1alpha1

import (
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// This file looks almost the same for all controllers
// Modify the addKnownTypes function, then run `make generate`

const (
	// ExtendedStatefulSetResourceKind is the kind name of ExtendedStatefulSet
	ExtendedStatefulSetResourceKind = "ExtendedStatefulSet"
	// ExtendedStatefulSetResourcePlural is the plural name of ExtendedStatefulSet
	ExtendedStatefulSetResourcePlural = "extendedstatefulsets"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is used for schema registrations in the controller package
	// and also in the generated kube code
	AddToScheme = schemeBuilder.AddToScheme

	// ExtendedStatefulSetResourceShortNames is the short names of ExtendedStatefulSet
	ExtendedStatefulSetResourceShortNames = []string{"ests"}

	// ExtendedStatefulSetValidation is the validation method for ExtendedStatefulSet
	ExtendedStatefulSetValidation = extv1.CustomResourceValidation{
		OpenAPIV3Schema: &extv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]extv1.JSONSchemaProps{
				"spec": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"template": {
							Type:        "object",
							Description: "A template for a regular StatefulSet",
						},
						"updateOnConfigChange": {
							Type:        "boolean",
							Description: "Indicate whether to update Pods in the StatefulSet when an env value or mount changes",
						},
						"zoneNodeLabel": {
							Type:        "string",
							Description: "Indicates the node label that a node locates",
						},
						"zones": {
							Type:        "array",
							Description: "Indicates the availability zones that the ExtendedStatefulSet needs to span",
							Items: &extv1.JSONSchemaPropsOrArray{
								Schema: &extv1.JSONSchemaProps{
									Type: "string",
								},
							},
						},
					},
					Required: []string{
						"template",
					},
				},
			},
		},
	}

	// ExtendedStatefulSetResourceName is the resource name of ExtendedStatefulSet
	ExtendedStatefulSetResourceName = fmt.Sprintf("%s.%s", ExtendedStatefulSetResourcePlural, apis.GroupName)

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
		&ExtendedStatefulSet{},
		&ExtendedStatefulSetList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
