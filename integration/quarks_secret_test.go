package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	inmemorygenerator "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("QuarksSecret", func() {
	var (
		qSecret qsv1a1.QuarksSecret
	)

	Context("when correctly setup", func() {
		BeforeEach(func() {
			qsName := fmt.Sprintf("qs-%s", helper.RandString(5))
			qSecret = env.DefaultQuarksSecret(qsName)
		})

		It("generates a secret with a password and deletes it when being deleted", func() {
			// Create an QuarksSecret
			var qs *qsv1a1.QuarksSecret
			qSecret.Spec.SecretName = "generated-password-secret"
			qs, tearDown, err := env.CreateQuarksSecret(env.Namespace, qSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(qs).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.CollectSecret(env.Namespace, "generated-password-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["password"]).To(MatchRegexp("^\\w{64}$"))

			// delete quarksSecret
			err = env.DeleteQuarksSecret(env.Namespace, qSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-password-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})

		It("generates a secret with an rsa key and deletes it when being deleted", func() {
			// Create an quarksSecret
			var qs *qsv1a1.QuarksSecret
			qSecret.Spec.Type = "rsa"
			qSecret.Spec.SecretName = "generated-rsa-secret"
			qs, tearDown, err := env.CreateQuarksSecret(env.Namespace, qSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(qs).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.CollectSecret(env.Namespace, "generated-rsa-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["private_key"]).To(ContainSubstring("RSA PRIVATE KEY"))
			Expect(secret.Data["public_key"]).To(ContainSubstring("PUBLIC KEY"))

			// delete quarksSecret
			err = env.DeleteQuarksSecret(env.Namespace, qSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-rsa-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})

		It("generates a secret with an ssh key and deletes it when being deleted", func() {
			// Create an quarksSecret
			var qs *qsv1a1.QuarksSecret
			qSecret.Spec.Type = "ssh"
			qSecret.Spec.SecretName = "generated-ssh-secret"
			qs, tearDown, err := env.CreateQuarksSecret(env.Namespace, qSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(qs).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.CollectSecret(env.Namespace, "generated-ssh-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["private_key"]).To(ContainSubstring("RSA PRIVATE KEY"))
			Expect(secret.Data["public_key"]).To(ContainSubstring("ssh-rsa "))
			Expect(secret.Data["public_key_fingerprint"]).To(MatchRegexp("([0-9a-f]{2}:){15}[0-9a-f]{2}"))

			// delete quarksSecret
			err = env.DeleteQuarksSecret(env.Namespace, qSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-ssh-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})

		It("generates a secret with a certificate key and deletes it when being deleted", func() {
			generator := inmemorygenerator.NewInMemoryGenerator(env.Log)
			ca, err := generator.GenerateCertificate("default-ca", credsgen.CertificateGenerationRequest{
				CommonName: "Fake CA",
				IsCA:       true,
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
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// Create an quarksSecret
			var qs *qsv1a1.QuarksSecret
			qSecret.Spec.SecretName = "generated-cert-secret"
			qSecret.Spec.Type = "certificate"
			qSecret.Spec.Request.CertificateRequest.CommonName = "example.com"
			qSecret.Spec.Request.CertificateRequest.CARef = qsv1a1.SecretReference{Name: "mysecret", Key: "ca"}
			qSecret.Spec.Request.CertificateRequest.CAKeyRef = qsv1a1.SecretReference{Name: "mysecret", Key: "key"}
			qSecret.Spec.Request.CertificateRequest.AlternativeNames = []string{"qux.com"}
			qs, tearDown, err = env.CreateQuarksSecret(env.Namespace, qSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(qs).NotTo(Equal(nil))
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// check for generated secret
			secret, err := env.CollectSecret(env.Namespace, "generated-cert-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["certificate"]).To(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(secret.Data["private_key"]).To(ContainSubstring("RSA PRIVATE KEY"))

			// delete QuarksSecret
			err = env.DeleteQuarksSecret(env.Namespace, qSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-cert-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})
	})
})
