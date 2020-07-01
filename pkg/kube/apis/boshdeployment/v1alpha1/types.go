package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// ReferenceType lists all the types of Reference we can supports
type ReferenceType = string

// Valid values for ref types
const (
	// ConfigMapReference represents ConfigMap reference
	ConfigMapReference ReferenceType = "configmap"
	// SecretReference represents Secret reference
	SecretReference ReferenceType = "secret"
	// URLReference represents URL reference
	URLReference ReferenceType = "url"

	ManifestSpecName        string = "manifest"
	OpsSpecName             string = "ops"
	ImplicitVariableKeyName string = "value"
)

// DeploymentSecretType lists all the types of secrets used in
// the lifecycle of a BOSHDeployment
type DeploymentSecretType int

const (
	// DeploymentSecretTypeManifestWithOps is a manifest that has ops files applied
	DeploymentSecretTypeManifestWithOps DeploymentSecretType = iota
	// DeploymentSecretTypeDesiredManifest is a manifest whose variables have been interpolated
	DeploymentSecretTypeDesiredManifest
	// DeploymentSecretTypeVariable is a BOSH variable generated using an QuarksSecret
	DeploymentSecretTypeVariable
	// DeploymentSecretTypeInstanceGroupResolvedProperties is a YAML file containing all properties needed to render an Instance Group
	DeploymentSecretTypeInstanceGroupResolvedProperties
	// DeploymentSecretBPMInformation is a YAML file containing the BPM information for one instance group
	DeploymentSecretBPMInformation
)

func (s DeploymentSecretType) String() string {
	return [...]string{
		"with-ops",
		"desired",
		"var",
		"ig-resolved",
		"bpm"}[s]
}

// Prefix returns the prefix used for our k8s secrets:
// `<secretType>.`
func (s DeploymentSecretType) Prefix() string {
	return s.String() + "."
}

var (
	// LabelDeploymentName is the label key for the deployment manifest name
	LabelDeploymentName = fmt.Sprintf("%s/deployment-name", apis.GroupName)
	// LabelDeploymentSecretType is the label key for secret type
	LabelDeploymentSecretType = fmt.Sprintf("%s/secret-type", apis.GroupName)
	// LabelInstanceGroupName is the name of a label for an instance group name.
	LabelInstanceGroupName = fmt.Sprintf("%s/instance-group-name", apis.GroupName)
	// LabelDeploymentVersion is the name of a label for the deployment's version.
	LabelDeploymentVersion = fmt.Sprintf("%s/deployment-version", apis.GroupName)
	// LabelReferencedJobName is the name key for dependent job
	LabelReferencedJobName = fmt.Sprintf("%s/referenced-job-name", apis.GroupName)
	// AnnotationLinkProvidesKey is the key for the quarks links 'provides' JSON on a secret
	AnnotationLinkProvidesKey = fmt.Sprintf("%s/provides", apis.GroupName)
	// AnnotationLinkProviderName is the annotation key used on services to identify the link it provides addresses for
	AnnotationLinkProviderName = fmt.Sprintf("%s/link-provider-name", apis.GroupName)
	// LabelEntanglementKey to identify a quarks link
	LabelEntanglementKey = fmt.Sprintf("%s/entanglement", apis.GroupName)
)

// BOSHDeploymentSpec defines the desired state of BOSHDeployment
type BOSHDeploymentSpec struct {
	Manifest ResourceReference   `json:"manifest"`
	Ops      []ResourceReference `json:"ops,omitempty"`
	Vars     []VarReference      `json:"vars,omitempty"`
}

// VarReference represents a user-defined secret for an explicit variable
type VarReference struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

// ResourceReference defines the resource reference type and location
type ResourceReference struct {
	Name string        `json:"name"`
	Type ReferenceType `json:"type"`
}

// BOSHDeploymentStatus defines the observed state of BOSHDeployment
type BOSHDeploymentStatus struct {
	// Timestamp for the last reconcile
	LastReconcile *metav1.Time `json:"lastReconcile"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BOSHDeployment is the Schema for the boshdeployments API
// +k8s:openapi-gen=true
type BOSHDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BOSHDeploymentSpec   `json:"spec,omitempty"`
	Status BOSHDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BOSHDeploymentList contains a list of BOSHDeployment
type BOSHDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BOSHDeployment `json:"items"`
}

// GetNamespacedName returns the resource name with its namespace
func (bdpl *BOSHDeployment) GetNamespacedName() string {
	return fmt.Sprintf("%s/%s", bdpl.Namespace, bdpl.Name)
}
