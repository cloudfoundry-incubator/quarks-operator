package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("QuarksLink from native provider to explicit links in BOSH", func() {
	const (
		manifestRef    = "manifest"
		deploymentName = "test"
	)

	var (
		tearDowns    []machine.TearDownFunc
		boshManifest corev1.Secret
		provider     corev1.Secret
		service      corev1.Service
	)

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	JustBeforeEach(func() {
		By("Creating a secret to provide values")
		tearDown, err := env.CreateSecret(env.Namespace, provider)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		By("Creating a service for the provider")
		tearDown, err = env.CreateService(env.Namespace, service)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		By("Creating the BOSH manifest")
		tearDown, err = env.CreateSecret(env.Namespace, boshManifest)
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)

		By("Creating the BOSH deployment")
		_, tearDown, err = env.CreateBOSHDeployment(env.Namespace,
			env.SecretBOSHDeployment(deploymentName, manifestRef))
		Expect(err).NotTo(HaveOccurred())
		tearDowns = append(tearDowns, tearDown)
	})

	Context("when deployment has a service with a selector", func() {
		BeforeEach(func() {
			By("Creating a minimal nats config")
			tearDown, err := env.CreateConfigMap(env.Namespace, env.NatsConfigMap(deploymentName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("Starting the nats pod")
			tearDown, err = env.CreatePod(env.Namespace, env.NatsPod(deploymentName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			// choose a manifest which uses the values from nats, i.e. the nats smoke test
			boshManifest = env.BOSHManifestSecret(manifestRef, bm.NatsSmokeTestWithExternalLinks)
			provider = env.NatsSecret(deploymentName)
			service = env.NatsService(deploymentName)
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

	Context("when deployment has a service with an endpoint", func() {
		var ep corev1.Endpoints
		BeforeEach(func() {
			ep = env.NatsEndpoints(deploymentName)
			tearDown, err := env.CreateEndpoints(env.Namespace, ep)
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			// choose a manifest which uses the values from nats, i.e. the nats smoke test
			boshManifest = env.BOSHManifestSecret(manifestRef, bm.NatsSmokeTestWithExternalLinks)
			provider = env.NatsSecret(deploymentName)
			service = env.NatsServiceForEndpoint(deploymentName)
		})

		It("uses the values provided by the native resources", func() {
			By("checking the ig manifest", func() {
				ig, err := env.CollectSecret(env.Namespace, "ig-resolved.nats-smoke-tests-v1")
				Expect(err).NotTo(HaveOccurred())

				igm := string(ig.Data["properties.yaml"])
				Expect(igm).To(ContainSubstring("address: nats-ep." + env.Namespace + ".svc."))
				Expect(igm).To(ContainSubstring("address: 192.168.0.1"))
			})

			By("updating the endpoint and checking the ig manifest", func() {
				ep.Subsets[0].Addresses[0].IP = "10.10.10.11"
				_, _, err := env.UpdateEndpoints(env.Namespace, ep)
				Expect(err).NotTo(HaveOccurred())

				ig, err := env.CollectSecret(env.Namespace, "ig-resolved.nats-smoke-tests-v2")
				Expect(err).NotTo(HaveOccurred())

				igm := string(ig.Data["properties.yaml"])
				Expect(igm).To(ContainSubstring("address: 10.10.10.11"))
			})
		})
	})

	Context("when deployment has a service with an external name", func() {
		BeforeEach(func() {
			manifest := bm.NatsSmokeTestWithExternalLinks
			boshManifest = env.BOSHManifestSecret(manifestRef, manifest)
			provider = env.NatsSecret(deploymentName)
			service = env.NatsServiceExternalName(deploymentName)
		})

		It("uses the values provided by the native resources", func() {
			By("checking the ig manifest", func() {
				ig, err := env.CollectSecret(env.Namespace, "ig-resolved.nats-smoke-tests-v1")
				Expect(err).NotTo(HaveOccurred())

				igm := string(ig.Data["properties.yaml"])
				Expect(igm).To(ContainSubstring("address: nats-headless." + env.Namespace + ".svc."))
				Expect(igm).To(ContainSubstring(`id: nats`))
				Expect(igm).To(ContainSubstring(`name: nats`))
				Expect(igm).To(ContainSubstring("password: r9fXAlY3gZ"))
				Expect(igm).To(ContainSubstring(`port: "4222"`))
				Expect(igm).To(ContainSubstring(`user: nats_client`))
			})
		})
	})
})
