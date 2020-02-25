package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("VersionedSecret", func() {
	var (
		secret    corev1.Secret
		tearDowns []machine.TearDownFunc
	)

	const (
		secretName = "versioned-secret"
	)

	BeforeEach(func() {
		secret = env.VersionedSecret(secretName)
		tearDown, err := env.CreateSecret(env.Namespace, secret)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		_, err = env.CollectSecret(env.Namespace, secretName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	Context("when updating data", func() {
		BeforeEach(func() {
			secret.StringData["new"] = "value"
		})

		It("fails because versioned secrets are immutable", func() {
			_, _, err := env.UpdateSecret(env.Namespace, secret)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("when updating annotations", func() {
		BeforeEach(func() {
			secret.Annotations = map[string]string{"foo": "bar"}
		})

		It("updates the annotation", func() {
			_, _, err := env.UpdateSecret(env.Namespace, secret)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("when updating annotation and data", func() {
		BeforeEach(func() {
			secret.StringData["new"] = "value"
			secret.Annotations = map[string]string{"foo": "bar"}
		})

		It("fails because versioned secrets are immutable", func() {
			_, _, err := env.UpdateSecret(env.Namespace, secret)
			Expect(err).To(HaveOccurred())
		})
	})
})
