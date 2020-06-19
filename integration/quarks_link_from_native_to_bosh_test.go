package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("QuarksLink from native to BOSH", func() {
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
		By("Creating the BOSH manifest")
		tearDown, err := env.CreateSecret(env.Namespace, boshManifest)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		By("Creating the BOSH deployment")
		_, tearDown, err = env.CreateBOSHDeployment(env.Namespace,
			env.SecretBOSHDeployment(deploymentName, manifestRef))
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)
	})

	Context("when deployment has explicit, external link dependencies", func() {
		BeforeEach(func() {
			By("Creating a minimal nats config")
			tearDown, err := env.CreateConfigMap(env.Namespace, env.NatsConfigMap(deploymentName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("Creating a secret to export values from that nats config")
			tearDown, err = env.CreateSecret(env.Namespace, env.NatsSecret(deploymentName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("Starting the nats pod")
			tearDown, err = env.CreatePod(env.Namespace, env.NatsPod(deploymentName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("Applying a service for the nats pod")
			tearDown, err = env.CreateService(env.Namespace, env.NatsService(deploymentName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			// choose a manifest which uses the values from nats, i.e. the nats smoke test
			boshManifest = env.BOSHManifestSecret(manifestRef, bm.NatsSmokeTestWithExternalLinks)
		})

		It("uses the values from the native resources", func() {
			By("waiting for job rendering done", func() {
				err := env.WaitForPods(env.Namespace, "quarks.cloudfoundry.org/instance-group-name=nats-smoke-tests")
				Expect(err).NotTo(HaveOccurred())
			})

			nats, err := env.GetPod(env.Namespace, "nats")
			Expect(err).NotTo(HaveOccurred())

			By("checking the ig manifest", func() {
				ig, err := env.GetSecret(env.Namespace, "ig-resolved.nats-smoke-tests-v1")
				Expect(err).NotTo(HaveOccurred())
				igm := string(ig.Data["properties.yaml"])

				Expect(igm).To(ContainSubstring("address: nats-headless." + env.Namespace + ".svc."))

				// instance address
				Expect(igm).To(ContainSubstring("address: " + nats.Status.PodIP))
				Expect(igm).To(ContainSubstring(`az: ""`))
				Expect(igm).To(ContainSubstring(`bootstrap: true`))
				Expect(igm).To(ContainSubstring(`id: `))
				Expect(igm).To(ContainSubstring(`index: 0`))
				Expect(igm).To(ContainSubstring(`name: nats`))

				// properties
				Expect(igm).To(ContainSubstring("password: r9fXAlY3gZ"))
				Expect(igm).To(ContainSubstring(`port: "4222"`))
				Expect(igm).To(ContainSubstring(`user: nats_client`))
			})

			By("updating the secret and checking the ig manifest again", func() {
				_, _, err = env.UpdateSecret(env.Namespace, env.NatsOtherSecret(deploymentName))
				Expect(err).NotTo(HaveOccurred())

				ig, err := env.CollectSecret(env.Namespace, "ig-resolved.nats-smoke-tests-v2")
				Expect(err).NotTo(HaveOccurred())
				igm := string(ig.Data["properties.yaml"])
				Expect(igm).To(ContainSubstring(`user: nats_user`))
				Expect(igm).To(ContainSubstring("password: abcdefg123"))
				Expect(igm).To(ContainSubstring(`port: "4222"`))
			})

			By("updating the service to an ExternalName service and checking the ig manifest", func() {
				svc, err := env.GetService(env.Namespace, "nats-headless")
				Expect(err).NotTo(HaveOccurred())
				svc.Spec = env.NatsServiceExternalName(deploymentName).Spec
				_, _, err = env.UpdateService(env.Namespace, *svc)
				Expect(err).NotTo(HaveOccurred())

				ig, err := env.CollectSecret(env.Namespace, "ig-resolved.nats-smoke-tests-v3")
				Expect(err).NotTo(HaveOccurred())
				igm := string(ig.Data["properties.yaml"])
				Expect(igm).NotTo(ContainSubstring("address: " + nats.Status.PodIP))
				Expect(igm).To(ContainSubstring(`id: nats`))
			})
		})
	})
})
