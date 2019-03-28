package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// Valid values for ref types
const (
	ManifestSpecName        string = "manifest"
	OpsSpecName             string = "ops"
	ConfigMapType           string = "configmap"
	SecretType              string = "secret"
	URLType                 string = "url"
	InterpolatedManifestKey string = "interpolated-manifest.yaml"
)

var (
	// LabelKind is the label key for manifest/job kind
	LabelKind = fmt.Sprintf("%s/kind", apis.GroupName)
	// LabelDeployment is the label key for manifest name
	LabelDeployment = fmt.Sprintf("%s/deployment", apis.GroupName)
	// LabelManifestSHA1 is the label key for manifest SHA1
	LabelManifestSHA1 = fmt.Sprintf("%s/manifestsha1", apis.GroupName)
	// AnnotationManifestSHA1 is the annotation key for manifest SHA1
	AnnotationManifestSHA1 = fmt.Sprintf("%s/manifestsha1", apis.GroupName)
)

// BOSHDeploymentSpec defines the desired state of BOSHDeployment
type BOSHDeploymentSpec struct {
	Manifest Manifest `json:"manifest"`
	Ops      []Ops    `json:"ops,omitempty"`
}

// Manifest defines the manifest type and location
type Manifest struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

// Ops defines the ops type and location
type Ops struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
}

// BOSHDeploymentStatus defines the observed state of BOSHDeployment
type BOSHDeploymentStatus struct {
	State string   `json:"state"`
	Nodes []string `json:"nodes"`
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
