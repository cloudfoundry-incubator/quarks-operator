package inmemorygenerator

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"github.com/cloudflare/cfssl/cli/genkey"
	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/pkg/errors"
)

const (
	inClusterKubeCACertPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// GenerateCertificate generates a certificate using Cloudflare's TLS toolkit
func (g InMemoryGenerator) GenerateCertificate(name string, request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
	g.log.Debugf("Generating certificate %s", name)
	cfssllog.Level = cfssllog.LevelWarning

	var certificate credsgen.Certificate
	var err error

	if request.IsCA {
		certificate, err = g.generateCACertificate(request)
		if err != nil {
			return credsgen.Certificate{}, errors.Wrap(err, "Generating CA certificate failed.")
		}
	} else {
		certificate, err = g.generateCertificate(request)
		if err != nil {
			return credsgen.Certificate{}, errors.Wrap(err, "Generating certificate failed.")
		}
	}
	return certificate, nil
}

// GenerateCertificateSigningRequest Generates a certificate signing request and private key
func (g InMemoryGenerator) GenerateCertificateSigningRequest(request credsgen.CertificateGenerationRequest) ([]byte, []byte, error) {
	cfssllog.Level = cfssllog.LevelWarning

	var csReq, privateKey []byte

	// Generate certificate request
	certReq := &csr.CertificateRequest{KeyRequest: &csr.BasicKeyRequest{A: g.Algorithm, S: g.Bits}}

	certReq.Hosts = append(certReq.Hosts, request.CommonName)
	certReq.Hosts = append(certReq.Hosts, request.AlternativeNames...)
	certReq.CN = certReq.Hosts[0]

	sslValidator := &csr.Generator{Validator: genkey.Validator}
	csReq, privateKey, err := sslValidator.ProcessRequest(certReq)
	if err != nil {
		return csReq, privateKey, err
	}
	return csReq, privateKey, nil
}

// generateCertificate Generate a local-issued certificate and private key
func (g InMemoryGenerator) generateCertificate(request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
	if !request.CA.IsCA {
		return credsgen.Certificate{}, errors.Errorf("The passed CA is not a CA")
	}

	cert := credsgen.Certificate{
		IsCA: false,
	}

	// Generate certificate
	signingReq, privateKey, err := g.GenerateCertificateSigningRequest(request)
	if err != nil {
		return credsgen.Certificate{}, err
	}

	// Parse CA
	caCert, err := helpers.ParseCertificatePEM([]byte(request.CA.Certificate))
	if err != nil {
		return credsgen.Certificate{}, errors.Wrap(err, "Parsing CA PEM failed.")
	}
	caKey, err := helpers.ParsePrivateKeyPEM([]byte(request.CA.PrivateKey))
	if err != nil {
		return credsgen.Certificate{}, errors.Wrap(err, "Parsing CA private key failed.")
	}

	// Sign certificate
	signingProfile := &config.SigningProfile{
		Usage:        []string{"server auth", "client auth"},
		Expiry:       time.Duration(g.Expiry*24) * time.Hour,
		ExpiryString: fmt.Sprintf("%dh", g.Expiry*24),
	}
	policy := &config.Signing{
		Profiles: map[string]*config.SigningProfile{},
		Default:  signingProfile,
	}

	s, err := local.NewSigner(caKey, caCert, signer.DefaultSigAlgo(caKey), policy)
	if err != nil {
		return credsgen.Certificate{}, errors.Wrap(err, "Creating signer failed.")
	}

	cert.Certificate, err = s.Sign(signer.SignRequest{Request: string(signingReq)})
	if err != nil {
		return credsgen.Certificate{}, errors.Wrap(err, "Signing certificate failed.")
	}
	cert.PrivateKey = privateKey

	return cert, nil
}

// generateCACertificate Generate self-signed root CA certificate and private key
func (g InMemoryGenerator) generateCACertificate(request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
	req := &csr.CertificateRequest{
		CA:         &csr.CAConfig{Expiry: fmt.Sprintf("%dh", g.Expiry*24)},
		CN:         request.CommonName,
		KeyRequest: &csr.BasicKeyRequest{A: g.Algorithm, S: g.Bits},
	}
	ca, _, privateKey, err := initca.New(req)
	if err != nil {
		return credsgen.Certificate{}, err
	}

	// If available, concatenate the CA cert with the in-cluster
	// Kubernetes ca cert
	if _, err := os.Stat(inClusterKubeCACertPath); err == nil {
		inClusterCABytes, err := ioutil.ReadFile(inClusterKubeCACertPath)
		if err != nil {
			return credsgen.Certificate{}, errors.Wrap(err, "failed to read in-cluster config")
		}

		ca = append(append(ca, []byte("\n")...), inClusterCABytes...)
	}

	cert := credsgen.Certificate{
		IsCA:        true,
		Certificate: ca,
		PrivateKey:  privateKey,
	}

	return cert, nil

}
