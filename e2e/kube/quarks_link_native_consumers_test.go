package kube_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/quarksrestart"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

var _ = Describe("BOSH deployment provides links to native k8s resources", func() {
	kubectl = cmdHelper.NewKubectl()

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

	BeforeEach(func() {
		err := apply("quarks-link/boshdeployment.yaml")
		Expect(err).ToNot(HaveOccurred())
		podWait("pod/nats-0")

	})

	Context("when creating a bosh deployment", func() {
		It("creates secrets for a all BOSH links", func() {
			exist, err := kubectl.SecretExists(namespace, "link-nats-nats")
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
					quarksrestart.RestartKey,
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
					quarksrestart.RestartKey,
				)
				Expect(err).ToNot(HaveOccurred(), "waiting for restart annotation on entangled pod")

				Expect(checkEntanglement(podName, "cat /quarks/link/nats-deployment/nats-nats/nats.password", "qwerty1234")).ToNot(HaveOccurred())
				Expect(checkEntanglement(podName, "echo $LINK_NATS_USER", "admin")).ToNot(HaveOccurred())
			})
		})
	})
})
