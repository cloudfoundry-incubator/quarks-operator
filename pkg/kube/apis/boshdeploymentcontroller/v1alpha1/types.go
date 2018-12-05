package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// Valid values for ref types
const (
	ManifestSpecName string = "manifest"
	OpsSpecName      string = "ops"
	ConfigMapType    string = "configmap"
	SecretType       string = "secret"
	URLType          string = "url"
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
