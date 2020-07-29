package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	controller "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/boshdeployment"
	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"

	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("BDPL Status", func() {
	const (
		deploymentName = "test"
		manifestName   = "manifest"

		opOneInstance = `- type: replace
  path: /instance_groups/name=quarks-gora?/instances
  value: 1
`
		opInstances = `- type: replace
  path: /instance_groups/name=quarks-gora?/instances
  value: 3
`
	)

	var tearDowns []machine.TearDownFunc

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	Context("when using the default configuration", func() {

		It("should deploy manifest and report the bdpl status", func() {
			var bdpl *bdv1.BOSHDeployment
			tearDown, err := env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMap(manifestName, bm.Gora))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			tearDown, err = env.CreateConfigMap(env.Namespace, env.CustomOpsConfigMap("bosh-ops", opOneInstance))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			tearDown, err = env.CreateSecret(env.Namespace, env.CustomOpsSecret("bosh-ops-secret", opInstances))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.InterpolateBOSHDeployment(deploymentName, manifestName, "bosh-ops", "bosh-ops-secret"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, deploymentName, "quarks-gora", "1", 3)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			sts, err := env.GetStatefulSet(env.Namespace, "quarks-gora")
			Expect(err).NotTo(HaveOccurred(), "error getting statefulset for deployment")
			Expect(*sts.Spec.Replicas).To(BeEquivalentTo(3))

			bdpl, err = env.GetBOSHDeployment(env.Namespace, deploymentName)
			Expect(err).NotTo(HaveOccurred())
			Expect(bdpl.Status.DeployedInstanceGroups).To(BeEquivalentTo(1))
			Expect(bdpl.Status.TotalInstanceGroups).To(BeEquivalentTo(1))
			Expect(bdpl.Status.TotalJobCount).To(BeEquivalentTo(2))
			Expect(bdpl.Status.CompletedJobCount).To(BeEquivalentTo(2))

			Eventually(func() string {
				bdpl, err = env.GetBOSHDeployment(env.Namespace, deploymentName)
				Expect(err).NotTo(HaveOccurred())
				return bdpl.Status.State
			}).Should(BeEquivalentTo(controller.BDPLStateDeployed))
		})

	})

})
