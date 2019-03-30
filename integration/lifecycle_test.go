package integration_test

import (
	"code.cloudfoundry.org/cf-operator/integration/environment"
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Lifecycle", func() {
	var (
		boshDeployment bdcv1.BOSHDeployment
	)

	Context("when correctly setup", func() {
		BeforeEach(func() {
			boshDeployment = env.DefaultBOSHDeployment("testcr", "manifest")
		})

		It("should exercise a deployment lifecycle", func() {
			// Create BOSH manifest in config map
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// Create fissile custom resource
			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, boshDeployment)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			err = env.WaitForPod(env.Namespace, "my-manifest-nats-v1-0")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from initial deployment")
		})
	})

})
