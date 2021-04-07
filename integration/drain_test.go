package integration_test

import (
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("Drain", func() {
	var (
		boshDeployment bdv1.BOSHDeployment
	)
	BeforeEach(func() {
		boshDeployment = env.DefaultBOSHDeployment("test", "manifest")
	})

	When("deleting the deployment", func() {
		It("executes the job's drain scripts", func() {
			cm := env.BOSHManifestConfigMap("manifest", bm.NatsSmall)
			cm.Data["manifest"] = bm.Drains
			tearDown, err := env.CreateConfigMap(env.Namespace, cm)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, boshDeployment)
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "drains", 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
			err = env.WaitForPodReady(env.Namespace, "drains-0")
			Expect(err).NotTo(HaveOccurred())

			done := make(chan interface{})

			go func() {
				// Check for files created by the drain scripts.
				var preAssertionWg, postAssertionWg sync.WaitGroup
				preAssertionWg.Add(2)
				postAssertionWg.Add(2)
				go func() {
					defer GinkgoRecover()
					preAssertionWg.Done()
					Expect(env.WaitForPodContainerLogMsg(env.Namespace, "drains-0", "delaying-drain-job-drain-watch", "delaying-drain-job.log")).To(BeNil(), "error finding file created by drain script")
					postAssertionWg.Done()
				}()
				go func() {
					defer GinkgoRecover()
					preAssertionWg.Done()
					Expect(env.WaitForPodContainerLogMsg(env.Namespace, "drains-0", "failing-drain-job-drain-watch", "failing-drain-job.log")).To(BeNil(), "error finding file created by drain script")
					postAssertionWg.Done()
				}()
				preAssertionWg.Wait()
				go func() {
					_ = env.DeleteBOSHDeployment(env.Namespace, boshDeployment.Name)
				}()
				postAssertionWg.Wait()
				close(done)
			}()

			Eventually(done, env.PollTimeout).Should(BeClosed())
		})
	})
})
