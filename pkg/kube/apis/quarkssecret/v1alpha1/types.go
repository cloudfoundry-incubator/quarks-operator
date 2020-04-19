package v1alpha1

import (
	"fmt"

	certv1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

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
	// LocalSigner defines the local as certificate signer
	LocalSigner SignerType = "local"
	// ClusterSigner defines the cluster as certificate signer
	ClusterSigner SignerType = "cluster"
)

var (
	// LabelKind is the label key for secret kind
	LabelKind = fmt.Sprintf("%s/secret-kind", apis.GroupName)
	// AnnotationCopyOf is a label key for secrets that are copies of generated secrets
	AnnotationCopyOf = fmt.Sprintf("%s/secret-copy-of", apis.GroupName)
	// AnnotationCertSecretName is the annotation key for certificate secret name
	AnnotationCertSecretName = fmt.Sprintf("%s/cert-secret-name", apis.GroupName)
	// AnnotationQSecName is the annotation key for the name of the owning quarks secret
	AnnotationQSecName = fmt.Sprintf("%s/quarks-secret-name", apis.GroupName)
	// AnnotationQSecNamespace is the annotation key for quarks secret namespace
	AnnotationQSecNamespace = fmt.Sprintf("%s/quarks-secret-namespace", apis.GroupName)
	// LabelSecretRotationTrigger is set on a config map to trigger secret
	// rotation. If set, then creating the config map will trigger secret
	// rotation.
	LabelSecretRotationTrigger = fmt.Sprintf("%s/secret-rotation", apis.GroupName)
	// RotateQSecretListName is the name of the config map entry, which
	// contains a JSON array of quarks secret names to rotate
	RotateQSecretListName = "secrets"
)

const (
	// GeneratedSecretKind is the kind of generated secret
	GeneratedSecretKind = "generated"
)

// SecretReference specifies a reference to another secret
type SecretReference struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// ServiceReference specifies a reference to a service
type ServiceReference struct {
	Name string
}

// CertificateRequest specifies the details for the certificate generation
type CertificateRequest struct {
	CommonName                  string             `json:"commonName"`
	AlternativeNames            []string           `json:"alternativeNames"`
	IsCA                        bool               `json:"isCA"`
	CARef                       SecretReference    `json:"CARef"`
	CAKeyRef                    SecretReference    `json:"CAKeyRef"`
	SignerType                  SignerType         `json:"signerType,omitempty"`
	Usages                      []certv1.KeyUsage  `json:"usages"`
	ServiceRef                  []ServiceReference `json:"serviceRef"`
	ActivateEKSWorkaroundForSAN bool               `json:"activateEKSWorkaroundForSAN,omitempty"`
}

// Request specifies details for the secret generation
type Request struct {
	CertificateRequest CertificateRequest `json:"certificate"`
}

// QuarksSecretSpec defines the desired state of QuarksSecret
type QuarksSecretSpec struct {
	Type       SecretType             `json:"type"`
	Request    Request                `json:"request"`
	SecretName string                 `json:"secretName"`
	Copies     []types.NamespacedName `json:"copies"`
}

// QuarksSecretStatus defines the observed state of QuarksSecret
type QuarksSecretStatus struct {
	// Timestamp for the last reconcile
	LastReconcile *metav1.Time `json:"lastReconcile"`
	// Indicates if the secret has already been generated
	Generated bool `json:"generated"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuarksSecret is the Schema for the QuarksSecrets API
// +k8s:openapi-gen=true
type QuarksSecret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuarksSecretSpec   `json:"spec,omitempty"`
	Status QuarksSecretStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuarksSecretList contains a list of QuarksSecret
type QuarksSecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuarksSecret `json:"items"`
}

// GetNamespacedName returns the resource name with its namespace
func (qs *QuarksSecret) GetNamespacedName() string {
	return fmt.Sprintf("%s/%s", qs.Namespace, qs.Name)
}
