package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// Type defines the type of the generated secret
type Type string

// Valid values for ref types
const (
	Password    Type = "password"
	Certificate Type = "certificate"
	SSHKey      Type = "ssh"
	RSAKey      Type = "rsa"
)

// SecretReference specifies a reference to another secret
type SecretReference struct {
	Name string
	Key  string
}

// CertificateRequest specifies the details for the certificate generation
type CertificateRequest struct {
	CommonName       string          `json:"commonName"`
	AlternativeNames []string        `json:"alternativeNames"`
	IsCA             bool            `json:"isCA"`
	CARef            SecretReference `json:"CARef"`
	CAKeyRef         SecretReference `json:"CAKeyRef"`
}

// Request specifies details for the secret generation
type Request struct {
	CertificateRequest CertificateRequest `json:"certificate"`
}

// ExtendedSecretSpec defines the desired state of ExtendedSecret
type ExtendedSecretSpec struct {
	Type       Type    `json:"type"`
	Request    Request `json:"request"`
	SecretName string  `json:"secretName"`
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
