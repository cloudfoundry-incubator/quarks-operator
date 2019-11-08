package integration_test

import (
	"fmt"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	bm "code.cloudfoundry.org/cf-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("Lifecycle", func() {
	var (
		boshDeployment bdv1.BOSHDeployment
	)

	Context("when correctly setup", func() {
		BeforeEach(func() {
			boshDeployment = env.DefaultBOSHDeployment("test", "manifest")
		})

		It("should exercise a deployment lifecycle", func() {
			// Create BOSH manifest in config map
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// Create fissile custom resource
			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, boshDeployment)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			// check for services
			headlessService, err := env.GetService(env.Namespace, "test-nats")
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

			clusterIPService, err := env.GetService(env.Namespace, "test-nats-0")
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
			Expect(env.WaitForSubsetsExist(env.Namespace, "test-nats-0")).To(BeNil(), "timeout getting subsets of endpoints 'test-nats-0'")
			Expect(env.WaitForSubsetsExist(env.Namespace, "test-nats-1")).To(BeNil(), "timeout getting subsets of endpoints 'test-nats-1'")

			clusterIPService, err = env.GetService(env.Namespace, "test-nats-1")
			Expect(err).NotTo(HaveOccurred(), "error getting service for instance group")

			// Check link address
			Expect(env.WaitForPodContainerLogMsg(env.Namespace, "test-nats-v1-0", "nats-nats", fmt.Sprintf("Trying to connect to route on %s:4223", clusterIPService.Name))).To(BeNil(), "error getting logs for connecting nats route")
			Expect(env.WaitForPodContainerLogMatchRegexp(env.Namespace, "test-nats-v1-0", "nats-nats", fmt.Sprintf(`%s:4223 - [\w:]+ - Route connection created`, clusterIPService.Spec.ClusterIP))).To(BeNil(), "error getting logs for resolving nats route address")
		})

		It("executes the job's drain scripts", func() {
			cm := env.DefaultBOSHManifestConfigMap("manifest")
			cm.Data["manifest"] = bm.Drains
			tearDown, err := env.CreateConfigMap(env.Namespace, cm)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, boshDeployment)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "drains", "1", 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
			err = env.WaitForPodReady(env.Namespace, "test-drains-v1-0")
			Expect(err).NotTo(HaveOccurred())

			// Expect(env.WaitForPodContainerLogMsg(env.Namespace, "test-drains-v1-0", "delaying-drain-job-drain-watch", "ls: cannot access '/tmp/drain_logs': No such file or directory")).To(BeNil(), "error getting logs from drain_watch process")

			// Check for files created by the drain scripts.
			var preAssertionWg, postAssertionWg sync.WaitGroup
			preAssertionWg.Add(2)
			postAssertionWg.Add(2)
			go func() {
				preAssertionWg.Done()
				Expect(env.WaitForPodContainerLogMsg(env.Namespace, "test-drains-v1-0", "delaying-drain-job-drain-watch", "delaying-drain-job.log")).To(BeNil(), "error finding file created by drain script")
				postAssertionWg.Done()
			}()
			go func() {
				preAssertionWg.Done()
				Expect(env.WaitForPodContainerLogMsg(env.Namespace, "test-drains-v1-0", "failing-drain-job-drain-watch", "failing-drain-job.log")).To(BeNil(), "error finding file created by drain script")
				postAssertionWg.Done()
			}()
			preAssertionWg.Wait()
			go env.DeleteBOSHDeployment(env.Namespace, boshDeployment.Name)
			postAssertionWg.Wait()
		})
	})
})
