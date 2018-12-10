package inmemorygenerator

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"log"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"golang.org/x/crypto/ssh"
)

func (g InMemoryGenerator) GenerateSSHKey(name string) (credsgen.SSHKey, error) {
	log.Println("Generating SSH key ", name)

	// generate private key
	private, err := rsa.GenerateKey(rand.Reader, 4096)
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
