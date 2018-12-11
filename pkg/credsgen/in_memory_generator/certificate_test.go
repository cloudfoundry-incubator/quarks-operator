package inmemorygenerator_test

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	inmemorygenerator "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

var _ = Describe("InMemoryGenerator", func() {
	var (
		generator credsgen.Generator = inmemorygenerator.NewInMemoryGenerator()
	)

	Describe("GenerateCertificate", func() {
		Context("when generating a certificate", func() {
			var (
				request credsgen.CertificateGenerationRequest
				ca      credsgen.Certificate
			)

			BeforeEach(func() {
				ca, _ = generator.GenerateCertificate("testca", credsgen.CertificateGenerationRequest{IsCA: true})
				request = credsgen.CertificateGenerationRequest{
					IsCA: false,
					CA:   ca,
				}
			})

			It("fails if the passed CA is not a CA", func() {
				ca := credsgen.Certificate{
					IsCA: false,
				}

				request.CA = ca

				_, err := generator.GenerateCertificate("foo", request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not a CA"))
			})

			It("considers the common name", func() {
				request.CommonName = "foo.com"
				cert, err := generator.GenerateCertificate("foo", request)
				Expect(err).ToNot(HaveOccurred())

				parsedCert, err := parseCert(cert.Certificate)
				Expect(err).ToNot(HaveOccurred())

				Expect(parsedCert.IsCA).To(BeFalse())
				Expect(parsedCert.DNSNames).To(ContainElement(Equal("foo.com")))
			})

			It("considers the alternative names", func() {
				request.CommonName = "foo.com"
				request.AlternativeNames = []string{"bar.com", "baz.com"}
				cert, err := generator.GenerateCertificate("foo", request)
				Expect(err).ToNot(HaveOccurred())

				parsedCert, err := parseCert(cert.Certificate)
				Expect(err).ToNot(HaveOccurred())

				Expect(parsedCert.IsCA).To(BeFalse())
				Expect(len(parsedCert.DNSNames)).To(Equal(3))
				Expect(parsedCert.DNSNames).To(ContainElement(Equal("bar.com")))
				Expect(parsedCert.DNSNames).To(ContainElement(Equal("baz.com")))
			})
		})

		Context("when generating a CA", func() {
			var (
				request credsgen.CertificateGenerationRequest
			)

			BeforeEach(func() {
				request = credsgen.CertificateGenerationRequest{
					IsCA: true,
				}
			})

			It("creates a CA", func() {
				request.CommonName = "example.com"
				cert, err := generator.GenerateCertificate("foo", request)
				Expect(err).ToNot(HaveOccurred())

				parsedCert, err := parseCert(cert.Certificate)
				Expect(err).ToNot(HaveOccurred())

				Expect(parsedCert.IsCA).To(BeTrue())
				Expect(cert.PrivateKey).ToNot(BeEmpty())
			})
		})
	})
})

func parseCert(certificate []byte) (*x509.Certificate, error) {
	certBlob, _ := pem.Decode(certificate)
	if certBlob == nil {
		return nil, fmt.Errorf("Could not decode certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlob.Bytes)
	if err != nil {
		return nil, errors.Wrap(err, "Parsing certificate")
	}

	return cert, nil
}
