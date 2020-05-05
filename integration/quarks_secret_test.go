package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/quarks-operator/pkg/credsgen"
	inmemorygenerator "code.cloudfoundry.org/quarks-operator/pkg/credsgen/in_memory_generator"
	qsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("QuarksSecret", func() {
	var (
		qSecret qsv1a1.QuarksSecret
	)

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

	Context("when quarks secret is a certificate", func() {
		var (
			tearDowns []machine.TearDownFunc
		)

		const qsName = "qsec-cert"

		BeforeEach(func() {
			qSecret = env.CertificateQuarksSecret(qsName, "mysecret", "ca", "key")

			By("creating the CA and storing it in a secret")
			generator := inmemorygenerator.NewInMemoryGenerator(env.Log)
			ca, err := generator.GenerateCertificate("default-ca", credsgen.CertificateGenerationRequest{
				CommonName: "Fake CA",
				IsCA:       true,
			})
			Expect(err).ToNot(HaveOccurred())

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
			tearDowns = append(tearDowns, tearDown)
		})

		JustBeforeEach(func() {
			By("creating the quarks secret")
			var qs *qsv1a1.QuarksSecret
			qs, tearDown, err := env.CreateQuarksSecret(env.Namespace, qSecret)
			Expect(err).NotTo(HaveOccurred())
			Expect(qs).NotTo(Equal(nil))
			tearDowns = append(tearDowns, tearDown)
		})

		AfterEach(func() {
			Expect(env.TearDownAll(tearDowns)).To(Succeed())
		})

		It("creates the certificate", func() {
			By("checking for the generated secret")
			secret, err := env.CollectSecret(env.Namespace, "generated-cert-secret")
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["certificate"]).To(ContainSubstring("BEGIN CERTIFICATE"))
			Expect(secret.Data["private_key"]).To(ContainSubstring("RSA PRIVATE KEY"))

			By("waiting for the generated secret to disappear, when deleting the quarks secret")
			err = env.DeleteQuarksSecret(env.Namespace, qSecret.Name)
			Expect(err).NotTo(HaveOccurred())
			err = env.WaitForSecretDeletion(env.Namespace, "generated-cert-secret")
			Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")
		})

		Context("when signer type is cluster", func() {
			BeforeEach(func() {
				qSecret.Spec.Request.CertificateRequest.SignerType = qsv1a1.ClusterSigner
			})

			It("updates existing secret", func() {
				By("waiting for the generated secret")
				oldSecret, err := env.CollectSecret(env.Namespace, "generated-cert-secret")
				Expect(err).NotTo(HaveOccurred())

				By("deleting quarks secret and generated secret")
				env.DeleteQuarksSecret(env.Namespace, qSecret.Name)

				err = env.WaitForSecretDeletion(env.Namespace, "generated-cert-secret")
				Expect(err).NotTo(HaveOccurred(), "dependent secret not deleted")

				By("creating an updated qsec")
				qSecret.Spec.Request.CertificateRequest.AlternativeNames = []string{"qux.com", "example.org"}
				_, _, err = env.CreateQuarksSecret(env.Namespace, qSecret)
				Expect(err).NotTo(HaveOccurred())

				By("checking for a working generated secret")
				secret, err := env.CollectSecret(env.Namespace, "generated-cert-secret")
				Expect(err).NotTo(HaveOccurred())
				Expect(oldSecret.Data["ca"]).To(Equal(secret.Data["ca"]))
				Expect(oldSecret.Data["private_key"]).NotTo(Equal(secret.Data["private_key"]))
				Expect(oldSecret.Data["certificate"]).NotTo(Equal(secret.Data["certificate"]))
			})
		})
	})
})
