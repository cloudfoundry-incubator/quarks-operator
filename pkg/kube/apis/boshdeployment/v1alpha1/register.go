package v1alpha1

import (
	"fmt"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apis "code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

// This file looks almost the same for all controllers
// Modify the addKnownTypes function, then run `make generate`

const (
	// BOSHDeploymentResourceKind is the kind name of BOSHDeployment
	BOSHDeploymentResourceKind = "BOSHDeployment"
	// BOSHDeploymentResourcePlural is the plural name of BOSHDeployment
	BOSHDeploymentResourcePlural = "boshdeployments"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is used for schema registrations in the controller package
	// and also in the generated kube code
	AddToScheme = schemeBuilder.AddToScheme

	// BOSHDeploymentResourceShortNames is the short names of BOSHDeployment
	BOSHDeploymentResourceShortNames = []string{"bdpl", "bdpls"}

	// BOSHDeploymentValidation is the validation method for BOSHDeployment
	BOSHDeploymentValidation = extv1.CustomResourceValidation{
		OpenAPIV3Schema: &extv1.JSONSchemaProps{
			Type: "object",
			Properties: map[string]extv1.JSONSchemaProps{
				"spec": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"manifest": {
							Type: "object",
							Properties: map[string]extv1.JSONSchemaProps{
								"name": {
									Type:      "string",
									MinLength: pointers.Int64(1),
								},
								"type": {
									Type: "string",
									Enum: []extv1.JSON{
										{
											Raw: []byte(`"configmap"`),
										},
										{
											Raw: []byte(`"secret"`),
										},
										{
											Raw: []byte(`"url"`),
										},
									},
								},
							},
							Required: []string{
								"type",
								"name",
							},
						},
						"ops": {
							Type: "array",
							Items: &extv1.JSONSchemaPropsOrArray{
								Schema: &extv1.JSONSchemaProps{
									Type: "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"name": {
											Type:      "string",
											MinLength: pointers.Int64(1),
										},
										"type": {
											Type: "string",
											Enum: []extv1.JSON{
												{
													Raw: []byte(`"configmap"`),
												},
												{
													Raw: []byte(`"secret"`),
												},
												{
													Raw: []byte(`"url"`),
												},
											},
										},
									},
									Required: []string{
										"type",
										"name",
									},
								},
							},
						},
						"vars": {
							Type: "array",
							Items: &extv1.JSONSchemaPropsOrArray{
								Schema: &extv1.JSONSchemaProps{
									Type: "object",
									Properties: map[string]extv1.JSONSchemaProps{
										"name": {
											Type:      "string",
											MinLength: pointers.Int64(1),
										},
										"secret": {
											Type:      "string",
											MinLength: pointers.Int64(1),
										},
									},
									Required: []string{
										"secret",
										"name",
									},
								},
							},
						},
					},
					Required: []string{
						"manifest",
					},
				},
				"status": {
					Type: "object",
					Properties: map[string]extv1.JSONSchemaProps{
						"lastReconcile": {
							Type:     "string",
							Nullable: true,
						},
						"state": {
							Type: "string",
						},
						"message": {
							Type: "string",
						},
						"totalJobCount": {
							Type: "integer",
						},
						"completedJobCount": {
							Type: "integer",
						},
						"totalInstanceGroups": {
							Type: "integer",
						},
						"deployedInstanceGroups": {
							Type: "integer",
						},
						"stateTimestamp": {
							Type:     "string",
							Nullable: true,
						},
					},
				},
			},
		},
	}

	// BOSHDeploymentResourceName is the resource name of BOSHDeployment
	BOSHDeploymentResourceName = fmt.Sprintf("%s.%s", BOSHDeploymentResourcePlural, apis.GroupName)

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
		&BOSHDeployment{},
		&BOSHDeploymentList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}
