package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	inmemorygenerator "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	es "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ExtendedSecret", func() {
	var (
		extendedSecret es.ExtendedSecret
	)

	Context("when correctly setup", func() {
		BeforeEach(func() {
			esName := fmt.Sprintf("testes-%s", helper.RandString(5))
			extendedSecret = env.DefaultExtendedSecret(esName)
		})

		It("generates a secret with a password and deletes it when being deleted", func() {
			// Create an ExtendedSecret
			var es *es.ExtendedSecret
			extendedSecret.Spec.SecretName = "generated-password-secret"
			es, tearDown, err := env.CreateExtendedSecret(env.Namespace, extendedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(es).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.GetSecret(env.Namespace, "generated-password-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["password"]).To(MatchRegexp("^\\w{64}$"))

			// delete ExtendedSecret
			err = env.DeleteExtendedSecret(env.Namespace, extendedSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-password-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})

		It("generates a secret with an rsa key and deletes it when being deleted", func() {
			// Create an ExtendedSecret
			var es *es.ExtendedSecret
			extendedSecret.Spec.Type = "rsa"
			extendedSecret.Spec.SecretName = "generated-rsa-secret"
			es, tearDown, err := env.CreateExtendedSecret(env.Namespace, extendedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(es).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.GetSecret(env.Namespace, "generated-rsa-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["private_key"]).To(ContainSubstring("RSA PRIVATE KEY"))
			Expect(secret.Data["public_key"]).To(ContainSubstring("PUBLIC KEY"))

			// delete ExtendedSecret
			err = env.DeleteExtendedSecret(env.Namespace, extendedSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-rsa-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})

		It("generates a secret with an ssh key and deletes it when being deleted", func() {
			// Create an ExtendedSecret
			var es *es.ExtendedSecret
			extendedSecret.Spec.Type = "ssh"
			extendedSecret.Spec.SecretName = "generated-ssh-secret"
			es, tearDown, err := env.CreateExtendedSecret(env.Namespace, extendedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(es).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.GetSecret(env.Namespace, "generated-ssh-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["private_key"]).To(ContainSubstring("RSA PRIVATE KEY"))
			Expect(secret.Data["public_key"]).To(ContainSubstring("ssh-rsa "))
			Expect(secret.Data["public_key_fingerprint"]).To(MatchRegexp("([0-9a-f]{2}:){15}[0-9a-f]{2}"))

			// delete ExtendedSecret
			err = env.DeleteExtendedSecret(env.Namespace, extendedSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-ssh-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})

		It("generates a secret with a certificate key and deletes it when being deleted", func() {
			generator := inmemorygenerator.NewInMemoryGenerator(env.Log)
			ca, err := generator.GenerateCertificate("default-ca", credsgen.CertificateGenerationRequest{
				IsCA: true,
			})
			Expect(err).ToNot(HaveOccurred())

			// Create the CA secret
			casecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: env.Namespace,
				},
				Data: map[string][]byte{
					"ca":  ca.Certificate,
					"key": ca.PrivateKey,
				},
			}
			tearDown, err := env.CreateSecret(env.Namespace, casecret)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// Create an ExtendedSecret
			var extendedsecret *es.ExtendedSecret
			extendedSecret.Spec.SecretName = "generated-cert-secret"
			extendedSecret.Spec.Type = "certificate"
			extendedSecret.Spec.Request.CertificateRequest.CommonName = "example.com"
			extendedSecret.Spec.Request.CertificateRequest.CARef = es.SecretReference{Name: "mysecret", Key: "ca"}
			extendedSecret.Spec.Request.CertificateRequest.CAKeyRef = es.SecretReference{Name: "mysecret", Key: "key"}
			extendedSecret.Spec.Request.CertificateRequest.AlternativeNames = []string{"qux.com"}
			extendedsecret, tearDown, err = env.CreateExtendedSecret(env.Namespace, extendedSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(extendedsecret).NotTo(Equal(nil))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.GetSecret(env.Namespace, "generated-cert-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["certificate"]).To(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(secret.Data["private_key"]).To(ContainSubstring("RSA PRIVATE KEY"))

			// delete ExtendedSecret
			err = env.DeleteExtendedSecret(env.Namespace, extendedSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-cert-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})
	})
})
