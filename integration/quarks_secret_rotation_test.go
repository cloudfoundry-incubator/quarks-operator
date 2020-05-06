package integration_test

import (
	"bytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	qsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("QuarksSecretRotation", func() {
	var (
		qsec      qsv1a1.QuarksSecret
		secret    *corev1.Secret
		tearDowns []machine.TearDownFunc
	)

	const (
		qsecName   = "test.var-secret"
		secretName = "generated-secret"
	)

	BeforeEach(func() {
		qsec = env.DefaultQuarksSecret(qsecName)
		_, tearDown, err := env.CreateQuarksSecret(env.Namespace, qsec)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		secret, err = env.CollectSecret(env.Namespace, secretName)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	Context("when creating a rotation config", func() {
		var oldPassword []byte

		BeforeEach(func() {
			oldPassword = secret.Data["password"]

			rotationConfig := env.RotationConfig(qsecName)
			tearDown, err := env.CreateConfigMap(env.Namespace, rotationConfig)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
		})

		It("modifies quarks secret and a a new password is generated", func() {
			err := env.WaitForConfigMap(env.Namespace, "rotation-config1")
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForQuarksSecretChange(env.Namespace, qsecName, func(qs qsv1a1.QuarksSecret) bool {
				return qs.Status.Generated == nil || (qs.Status.Generated != nil && !*qs.Status.Generated)
			})
			Expect(err).NotTo(HaveOccurred())

			err = env.WaitForSecretChange(env.Namespace, secretName, func(s corev1.Secret) bool {
				return !bytes.Equal(oldPassword, s.Data["password"])
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
