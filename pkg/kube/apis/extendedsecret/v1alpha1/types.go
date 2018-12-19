package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

type Type string

// Valid values for ref types
const (
	Password    Type = "password"
	Certificate Type = "certificate"
	SSHKey      Type = "ssh"
	RSAKey      Type = "rsa"
)

type SecretReference struct {
	Name string
	Key  string
}

type CertificateRequest struct {
	CommonName       string          `json:"common_name"`
	AlternativeNames []string        `json:"alternative_names"`
	IsCA             bool            `json:"is_ca"`
	CARef            SecretReference `json:"ca_ref"`
	CAKeyRef         SecretReference `json:"ca_key_ref"`
}

type Request struct {
	CertificateRequest CertificateRequest `json:"certificate"`
}

// ExtendedSecretSpec defines the desired state of ExtendedSecret
type ExtendedSecretSpec struct {
	Type    Type    `json:"type"`
	Request Request `json:"request"`
}

// ExtendedSecretStatus defines the observed state of ExtendedSecret
type ExtendedSecretStatus struct {
	SecretStatus []string `json:"secretStatus"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedSecret is the Schema for the ExtendedSecrets API
// +k8s:openapi-gen=true
type ExtendedSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExtendedSecretSpec   `json:"spec,omitempty"`
	Status ExtendedSecretStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedSecretList contains a list of ExtendedSecret
type ExtendedSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExtendedSecret `json:"items"`
}
