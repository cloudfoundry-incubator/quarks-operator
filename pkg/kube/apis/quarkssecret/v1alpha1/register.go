package v1alpha1

import (
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apis "code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
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

	// QuarksSecretValidation is the validation schema for QuarksSecret
	QuarksSecretValidation = extv1.CustomResourceValidation{
		OpenAPIV3Schema: &extv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]extv1.JSONSchemaProps{
				"spec": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"secretName": {
							Type:        "string",
							MinLength:   pointers.Int64(1),
							Description: "The name of the generated secret",
						},
						"type": {
							Type:        "string",
							MinLength:   pointers.Int64(1),
							Description: "What kind of secret to generate: password, certificate, ssh, rsa",
						},
						"request": {
							Type:                   "object",
							XPreserveUnknownFields: pointers.Bool(true),
						},
						"copies": {
							Type:        "array",
							Description: "A list of namespaced names where to copy generated secrets",
							Items: &extv1.JSONSchemaPropsOrArray{
								Schema: &extv1.JSONSchemaProps{
									Type: "object",
								},
							},
						},
					},
					Required: []string{
						"secretName",
						"type",
					},
				},
				"status": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"generated": {
							Type: "boolean",
						},
						"lastReconcile": {
							Type: "string",
						},
					},
				},
			},
		},
	}

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
