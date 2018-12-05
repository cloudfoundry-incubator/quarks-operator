package credsgen

// PasswordGenerationRequest specifies the generation parameters for Passwords
type PasswordGenerationRequest struct {
	Length int
}

// CertificateGenerationRequest specifies the generation parameters for Certificates
type CertificateGenerationRequest struct {
	CommonName       string
	AlternativeNames []string
	IsCA             bool
	CA               string
}

// Certificate holds the information about a certificate
type Certificate struct {
	CA          string
	Certificate string
	PrivateKey  string
}

// SSHKey represents an SSH key
type SSHKey struct {
	PrivateKey  string
	PublicKey   string
	Fingerprint string
}

// RSAKey represents an RSA key
type RSAKey struct {
	PrivateKey string
	PublicKey  string
}

// Generator provides an interface for generating credentials like passwords, certificates or SSH and RSA keys
type Generator interface {
	GeneratePassword(PasswordGenerationRequest) string
	GenerateCertificate(CertificateGenerationRequest) Certificate
	GenerateSSHKey() SSHKey
	GenerateRSAKey() RSAKey
}
