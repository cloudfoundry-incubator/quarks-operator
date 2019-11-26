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
	// QuarksStatefulSetResourceKind is the kind name of QuarksStatefulSet
	QuarksStatefulSetResourceKind = "QuarksStatefulSet"
	// QuarksStatefulSetResourcePlural is the plural name of QuarksStatefulSet
	QuarksStatefulSetResourcePlural = "quarksstatefulsets"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is used for schema registrations in the controller package
	// and also in the generated kube code
	AddToScheme = schemeBuilder.AddToScheme

	// QuarksStatefulSetResourceShortNames is the short names of QuarksStatefulSet
	QuarksStatefulSetResourceShortNames = []string{"qsts"}

	// QuarksStatefulSetValidation is the validation method for QuarksStatefulSet
	QuarksStatefulSetValidation = extv1.CustomResourceValidation{
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
						"activePassiveProbe": {
							Type:        "object",
							Description: "Defines a probe to determine an active/passive component instance",
						},
						"zoneNodeLabel": {
							Type:        "string",
							Description: "Indicates the node label that a node locates",
						},
						"zones": {
							Type:        "array",
							Description: "Indicates the availability zones that the QuarksStatefulSet needs to span",
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

	// QuarksStatefulSetResourceName is the resource name of QuarksStatefulSet
	QuarksStatefulSetResourceName = fmt.Sprintf("%s.%s", QuarksStatefulSetResourcePlural, apis.GroupName)

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
		&QuarksStatefulSet{},
		&QuarksStatefulSetList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
