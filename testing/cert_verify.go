package testing

import (
	"crypto/x509"
	"encoding/pem"

	"github.com/pkg/errors"
)

// CertificateVerify verifies certificate with root certificate.
func CertificateVerify(rootPEM, certPEM []byte, dnsName string) error {
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(rootPEM)
	if !ok {
		return errors.Errorf("Could not parse rootPEM")
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.Errorf("Could not parse certPEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrapf(err, "Could not parse certificate")
	}

	opts := x509.VerifyOptions{
		DNSName: dnsName,
		Roots:   roots,
	}

	if _, err := cert.Verify(opts); err != nil {
		return errors.Wrapf(err, "Could not verify certificate")
	}

	return nil
}
