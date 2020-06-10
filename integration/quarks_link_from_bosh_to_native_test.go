package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("QuarksLink from BOSH to native", func() {
	const (
		manifestRef    = "manifest"
		deploymentName = "test"
	)

	var (
		tearDowns    []machine.TearDownFunc
		boshManifest corev1.Secret
	)

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	JustBeforeEach(func() {
		tearDown, err := env.CreateSecret(env.Namespace, boshManifest)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		_, tearDown, err = env.CreateBOSHDeployment(env.Namespace,
			env.SecretBOSHDeployment(deploymentName, manifestRef))
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)
	})

	Context("when deployment has implicit links only", func() {
		BeforeEach(func() {
			boshManifest = env.BOSHManifestSecret(manifestRef, bm.NatsSmall)
		})

		It("creates a secret for each link found in jobs", func() {
			secretName := names.QuarksLinkSecretName("nats", "nats")

			By("waiting for secrets", func() {
				err := env.WaitForSecret(env.Namespace, secretName)
				Expect(err).NotTo(HaveOccurred())
				secret, err := env.GetSecret(env.Namespace, secretName)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Data).To(HaveKeyWithValue(
					"nats.password", []byte("changeme")))
				Expect(secret.Data).To(HaveKeyWithValue(
					"nats.port", []byte("4222")))
				Expect(secret.Data).To(HaveKeyWithValue(
					"nats.user", []byte("admin")))
			})
		})
	})

	Context("when deployment has explicit links", func() {
		BeforeEach(func() {
			boshManifest = env.BOSHManifestSecret(manifestRef, bm.NatsSmallWithLinks)
		})

		It("creates a secret for each link found in jobs", func() {
			secretName := names.QuarksLinkSecretName("nats", "nutty-nuts")

			By("waiting for secrets", func() {
				err := env.WaitForSecret(env.Namespace, secretName)
				Expect(err).NotTo(HaveOccurred())
				secret, err := env.GetSecret(env.Namespace, secretName)
				Expect(err).NotTo(HaveOccurred())
				Expect(secret.Data).To(HaveKeyWithValue(
					"nats.password", []byte("changeme")))
				Expect(secret.Data).To(HaveKeyWithValue(
					"nats.port", []byte("4222")))
				Expect(secret.Data).To(HaveKeyWithValue(
					"nats.user", []byte("admin")))
			})
		})
	})
})
