package integration_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
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

			// Check pods
			Expect(env.WaitForLogMsg(env.ObservedLogs, "Considering 3 extended jobs for pod testcr-nats-v1-0/ready")).To(Succeed(), "error getting logs for waiting pod-0/ready")
			Expect(env.ObservedLogs.TakeAll())
			Expect(env.WaitForLogMsg(env.ObservedLogs, "Considering 3 extended jobs for pod testcr-nats-v1-1/ready")).To(Succeed(), "error getting logs for waiting pod-1/ready")

			err = env.WaitForPod(env.Namespace, "testcr-nats-v1-0")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from initial deployment")
			err = env.WaitForPod(env.Namespace, "testcr-nats-v1-1")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pod from initial deployment")

			// check for services
			headlessService, err := env.GetService(env.Namespace, "testcr-nats")
			Expect(err).NotTo(HaveOccurred(), "error getting service for instance group")
			Expect(headlessService.Spec.Ports)
			Expect(headlessService.Spec.Selector).To(Equal(map[string]string{
				bdm.LabelInstanceGroupName: "nats",
			}))
			Expect(headlessService.Spec.Ports[0].Name).To(Equal("nats"))
			Expect(headlessService.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(headlessService.Spec.Ports[0].Port).To(Equal(int32(4222)))
			Expect(headlessService.Spec.Ports[1].Name).To(Equal("nats-routes"))
			Expect(headlessService.Spec.Ports[1].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(headlessService.Spec.Ports[1].Port).To(Equal(int32(4223)))

			clusterIPService, err := env.GetService(env.Namespace, "testcr-nats-0")
			Expect(err).NotTo(HaveOccurred(), "error getting service for instance group")
			Expect(clusterIPService.Spec.Ports)
			Expect(clusterIPService.Spec.Selector).To(Equal(map[string]string{
				bdm.LabelInstanceGroupName: "nats",
				essv1.LabelAZIndex:         "0",
				essv1.LabelPodOrdinal:      "0",
			}))
			Expect(clusterIPService.Spec.Ports[0].Name).To(Equal("nats"))
			Expect(clusterIPService.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(clusterIPService.Spec.Ports[0].Port).To(Equal(int32(4222)))
			Expect(clusterIPService.Spec.Ports[1].Name).To(Equal("nats-routes"))
			Expect(clusterIPService.Spec.Ports[1].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(clusterIPService.Spec.Ports[1].Port).To(Equal(int32(4223)))

			// check for endpoints
			Expect(env.WaitForSubsetsExist(env.Namespace, "testcr-nats-0")).To(BeNil(), "timeout getting subsets of endpoints 'testcr-nats-0'")
			Expect(env.WaitForSubsetsExist(env.Namespace, "testcr-nats-1")).To(BeNil(), "timeout getting subsets of endpoints 'testcr-nats-1'")

			clusterIPService, err = env.GetService(env.Namespace, "testcr-nats-1")
			Expect(err).NotTo(HaveOccurred(), "error getting service for instance group")

			// Check link address
			Expect(env.WaitForPodLogMsg(env.Namespace, "testcr-nats-v1-0", fmt.Sprintf("Trying to connect to route on %s.%s.svc.cluster.local:4223", clusterIPService.Name, env.Namespace))).To(BeNil(), "error getting logs for connecting nats route")
			Expect(env.WaitForPodLogMatchRegexp(env.Namespace, "testcr-nats-v1-0", fmt.Sprintf(`%s:4223 - [\w:]+ - Route connection created`, clusterIPService.Spec.ClusterIP))).To(BeNil(), "error getting logs for resolving nats route address")
		})
	})

})
