package kube_test

import (
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/quarkslink"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

var _ = Describe("BOSHLinkEntanglements", func() {

	var (
		err error
	)
	apply := func(p string) error {
		yamlPath := path.Join(examplesDir, p)
		return cmdHelper.Apply(namespace, yamlPath)
	}
	kubectl = cmdHelper.NewKubectl()

	Context("testing native to bosh", func() {
		It("uses kube native link", func() {

			err = apply("quarks-link/link-pod.yaml")
			Expect(err).ToNot(HaveOccurred())
			err = apply("quarks-link/link-secret.yaml")
			Expect(err).ToNot(HaveOccurred())
			err = apply("quarks-link/link-service.yaml")
			Expect(err).ToNot(HaveOccurred())

			err = apply("quarks-link/boshdeployment-with-external-consumer.yaml")
			Expect(err).ToNot(HaveOccurred())

			podWait("pod/cf-operator-testing-deployment-draining-ig-0")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	checkEntanglement := func(podName, cmd, expect string) error {
		return kubectl.RunCommandWithCheckString(
			namespace, podName,
			cmd,
			expect,
		)
	}

	getPodName := func(selector string) string {
		podNames, err := kubectl.GetPodNames(namespace, selector)
		Expect(err).ToNot(HaveOccurred())
		Expect(podNames).To(HaveLen(1))
		return podNames[0]
	}

	Context("testing bosh to native", func() {

		BeforeEach(func() {
			err := apply("quarks-link/boshdeployment.yaml")
			Expect(err).ToNot(HaveOccurred())
			podWait("pod/nats-deployment-nats-0")

		})

		Context("when creating a bosh deployment", func() {
			It("creates secrets for a all BOSH links", func() {
				exist, err := kubectl.SecretExists(namespace, "link-nats-deployment-nats-nats")
				Expect(err).ToNot(HaveOccurred())
				Expect(exist).To(BeTrue())
			})
		})

		Context("when entangling a statefulsets pod", func() {
			It("supports entangled pods", func() {
				const (
					podName  = "entangled-statefulset-0"
					selector = "pod/entangled-statefulset-0"
				)

				By("mutating new pods to mount the secret", func() {
					err := apply("quarks-link/entangled-sts.yaml")
					Expect(err).ToNot(HaveOccurred())
					podWait(selector)

					Expect(checkEntanglement(podName, "cat /quarks/link/nats-deployment/nats-nats/nats.password", "onetwothreefour")).ToNot(HaveOccurred())
					Expect(checkEntanglement(podName, "echo $LINK_NATS_USER", "admin")).ToNot(HaveOccurred())
				})

				By("restarting pods when the link secret changes", func() {
					err := apply("quarks-link/password-ops.yaml")
					Expect(err).ToNot(HaveOccurred())
					err = kubectl.WaitForData(
						namespace, "pod", podName,
						`jsonpath="{.metadata.annotations}"`,
						quarkslink.RestartKey,
					)
					Expect(err).ToNot(HaveOccurred(), "waiting for restart annotation on entangled pod")
					podWait(selector)

					Expect(checkEntanglement(podName, "cat /quarks/link/nats-deployment/nats-nats/nats.password", "qwerty1234")).ToNot(HaveOccurred())
					Expect(checkEntanglement(podName, "echo $LINK_NATS_USER", "admin")).ToNot(HaveOccurred())
				})
			})
		})

		Context("when entangling a deployments pod", func() {
			It("supports entangled pods", func() {
				const selector = "example=owned-by-dpl"
				// pod names in deployments contain a dynamic suffix
				var podName string

				By("mutating new pods to mount the secret", func() {
					err := apply("quarks-link/entangled-dpl.yaml")
					Expect(err).ToNot(HaveOccurred())
					err = kubectl.WaitLabelFilter(namespace, "ready", "pod", selector)
					Expect(err).ToNot(HaveOccurred())

					podName = getPodName(selector)
					podWait("pod/" + podName)

					Expect(checkEntanglement(podName, "cat /quarks/link/nats-deployment/nats-nats/nats.password", "onetwothreefour")).ToNot(HaveOccurred())
					Expect(checkEntanglement(podName, "echo $LINK_NATS_USER", "admin")).ToNot(HaveOccurred())
				})

				By("restarting pods when the link secret changes", func() {
					err := apply("quarks-link/password-ops.yaml")
					Expect(err).ToNot(HaveOccurred())

					err = kubectl.WaitForPodDelete(namespace, podName)
					Expect(err).ToNot(HaveOccurred(), "waiting for old pod to terminate")

					err = kubectl.WaitLabelFilter(namespace, "ready", "pod", selector)
					Expect(err).ToNot(HaveOccurred())

					podName = getPodName(selector)
					err = kubectl.WaitForData(
						namespace, "pod", podName,
						`jsonpath="{.metadata.annotations}"`,
						quarkslink.RestartKey,
					)
					Expect(err).ToNot(HaveOccurred(), "waiting for restart annotation on entangled pod")

					Expect(checkEntanglement(podName, "cat /quarks/link/nats-deployment/nats-nats/nats.password", "qwerty1234")).ToNot(HaveOccurred())
					Expect(checkEntanglement(podName, "echo $LINK_NATS_USER", "admin")).ToNot(HaveOccurred())
				})
			})
		})
	})

})
