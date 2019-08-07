package v1alpha1

import (
	"fmt"

	certv1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// SecretType defines the type of the generated secret
type SecretType = string

// Valid values for secret types
const (
	Password    SecretType = "password"
	Certificate SecretType = "certificate"
	SSHKey      SecretType = "ssh"
	RSAKey      SecretType = "rsa"
)

// SignerType defines the type of the certificate signer
type SignerType = string

// Valid values for signer types
const (
	LocalSigner    SignerType = "local"
	ExternalSigner SignerType = "external"
)

var (
	// LabelKind is the label key for secret kind
	LabelKind = fmt.Sprintf("%s/secret-kind", apis.GroupName)
	// AnnotationCertSecretName is the annotation key for certificate secret name
	AnnotationCertSecretName = fmt.Sprintf("%s/cert-secret-name", apis.GroupName)
	// AnnotationExSecretNamespace is the annotation key for ex-secret-namespace
	AnnotationExSecretNamespace = fmt.Sprintf("%s/ex-secret-namespace", apis.GroupName)
)

const (
	// GeneratedSecretKind is the kind of generated secret
	GeneratedSecretKind = "generated"
)

// SecretReference specifies a reference to another secret
type SecretReference struct {
	Name string
	Key  string
}

// CertificateRequest specifies the details for the certificate generation
type CertificateRequest struct {
	CommonName       string            `json:"commonName"`
	AlternativeNames []string          `json:"alternativeNames"`
	IsCA             bool              `json:"isCA"`
	CARef            SecretReference   `json:"CARef"`
	CAKeyRef         SecretReference   `json:"CAKeyRef"`
	SignerType       SignerType        `json:"signerType,omitempty"`
	Usages           []certv1.KeyUsage `json:"usages,omitempty"`
}

// Request specifies details for the secret generation
type Request struct {
	CertificateRequest CertificateRequest `json:"certificate"`
}

// ExtendedSecretSpec defines the desired state of ExtendedSecret
type ExtendedSecretSpec struct {
	Type       SecretType `json:"type"`
	Request    Request    `json:"request"`
	SecretName string     `json:"secretName"`
}

// ExtendedSecretStatus defines the observed state of ExtendedSecret
type ExtendedSecretStatus struct {
	// Indicates if the secret has already been generated
	Generated bool `json:"generated"`
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
