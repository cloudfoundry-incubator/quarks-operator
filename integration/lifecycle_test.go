package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
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

			err = env.WaitForPod(env.Namespace, "testcr-nats-v1-0")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from initial deployment")

			// check for service
			svc, err := env.GetService(env.Namespace, "testcr-nats-headless")
			Expect(err).NotTo(HaveOccurred(), "error getting service for instance group")
			Expect(svc.Spec.Ports)
			Expect(svc.Spec.Selector).To(Equal(map[string]string{
				bdm.LabelInstanceGroupName: "nats",
			}))
			Expect(svc.Spec.Ports[0].Name).To(Equal("nats"))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(4222)))
		})
	})

})
