package inmemorygenerator

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"golang.org/x/crypto/ssh"
)

// GenerateSSHKey generates an SSH key using go's standard crypto library
func (g InMemoryGenerator) GenerateSSHKey(name string) (credsgen.SSHKey, error) {
	g.log.Debugf("Generating SSH key %s", name)

	// generate private key
	private, err := rsa.GenerateKey(rand.Reader, g.Bits)
	if err != nil {
		return credsgen.SSHKey{}, err
	}
	privateBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	}
	privatePEM := pem.EncodeToMemory(privateBlock)

	// Calculate public key
	public, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		return credsgen.SSHKey{}, err
	}

	key := credsgen.SSHKey{
		PrivateKey:  privatePEM,
		PublicKey:   ssh.MarshalAuthorizedKey(public),
		Fingerprint: ssh.FingerprintLegacyMD5(public),
	}
	return key, nil
}
