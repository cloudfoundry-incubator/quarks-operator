package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	es "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("ExtendedSecret", func() {
	var (
		extendedSecret es.ExtendedSecret
	)

	Context("when correctly setup", func() {
		BeforeEach(func() {
			esName := fmt.Sprintf("testes-%s", testing.RandString(5))
			extendedSecret = env.DefaultExtendedSecret(esName)
		})

		It("generates a secret with a password", func() {
			// Create an ExtendedSecret
			var es *es.ExtendedSecret
			es, tearDown, err := env.CreateExtendedSecret(env.Namespace, extendedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(es).NotTo(Equal(nil))
			defer tearDown()

			// check for generated secret
			secret, err := env.GetSecret(env.Namespace, "es-secret-"+extendedSecret.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["password"]).To(MatchRegexp("^\\w{64}$"))
		})

		It("generates a secret with an rsa key", func() {
			// Create an ExtendedSecret
			var es *es.ExtendedSecret
			extendedSecret.Spec.Type = "rsa"
			es, tearDown, err := env.CreateExtendedSecret(env.Namespace, extendedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(es).NotTo(Equal(nil))
			defer tearDown()

			// check for generated secret
			secret, err := env.GetSecret(env.Namespace, "es-secret-"+extendedSecret.GetName())
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["RSAPrivateKey"]).To(ContainSubstring("RSA PRIVATE KEY"))
			Expect(secret.Data["RSAPublicKey"]).To(ContainSubstring("PUBLIC KEY"))
		})
	})
})
