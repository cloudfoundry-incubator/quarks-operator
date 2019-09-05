package v1alpha1

import (
	"fmt"

	apis "code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// This file looks almost the same for all controllers
// Modify the addKnownTypes function, then run `make generate`

const (
	// ExtendedJobResourceKind is the kind name of ExtendedJob
	ExtendedJobResourceKind = "ExtendedJob"
	// ExtendedJobResourcePlural is the plural name of ExtendedJob
	ExtendedJobResourcePlural = "extendedjobs"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is used for schema registrations in the controller package
	// and also in the generated kube code
	AddToScheme = schemeBuilder.AddToScheme

	// ExtendedJobResourceShortNames is the short names of ExtendedJob
	ExtendedJobResourceShortNames = []string{"ejob", "ejobs"}
	// ExtendedJobValidation is the validation method for ExtendedJob
	ExtendedJobValidation = extv1.CustomResourceValidation{
		OpenAPIV3Schema: &extv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]extv1.JSONSchemaProps{
				"spec": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"output": {
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"namePrefix": {
									Type: "string",
								},
								"outputType": {
									Type: "string",
								},
								"secretLabels": {
									Type: "object",
								},
								"writeOnFailure": {
									Type: "boolean",
								},
							},
							Required: []string{
								"namePrefix",
							},
						},
						"trigger": {
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"strategy": {
									Type: "string",
									Enum: []extv1.JSON{
										{
											Raw: []byte(`"manual"`),
										},
										{
											Raw: []byte(`"once"`),
										},
										{
											Raw: []byte(`"now"`),
										},
										{
											Raw: []byte(`"done"`),
										},
									},
								},
								"when": {
									Type: "string",
									Enum: []extv1.JSON{
										{
											Raw: []byte(`"ready"`),
										},
										{
											Raw: []byte(`"notready"`),
										},
										{
											Raw: []byte(`"created"`),
										},
										{
											Raw: []byte(`"deleted"`),
										},
									},
								},
								"selector": {
									Type: "object",
								},
							},
							Required: []string{
								"strategy",
							},
						},
						"template": {
							Type: "object",
						},
						"updateOnConfigChange": {
							Type: "boolean",
						},
					},
				},
			},
		},
	}

	// ExtendedJobResourceName is the resource name of ExtendedJob
	ExtendedJobResourceName = fmt.Sprintf("%s.%s", ExtendedJobResourcePlural, apis.GroupName)

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
		&ExtendedJob{},
		&ExtendedJobList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
