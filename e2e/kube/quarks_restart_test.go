package kube_test

import (
	"path"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/quarksrestart"
	"code.cloudfoundry.org/quarks-utils/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("QuarksRestart", func() {

	getPodName := func(selector string) string {
		podNames, err := kubectl.GetPodNames(namespace, selector)
		Expect(err).ToNot(HaveOccurred())
		Expect(podNames).To(HaveLen(1))
		return podNames[0]
	}

	updateSecret := func() {
		f := path.Join(examplesDir, "quarks-restart/secret-updated.yaml")
		err := testing.Apply(namespace, f)
		Expect(err).ToNot(HaveOccurred())
	}

	Context("when a secret has quarks restart annotation", func() {
		kubectl := testing.NewKubectl()
		const selector = "app=sample"

		BeforeEach(func() {
			By("Creating secret")
			f := path.Join(examplesDir, "quarks-restart/secret.yaml")
			err := testing.Create(namespace, f)
			Expect(err).ToNot(HaveOccurred())
		})

		It("the deployment should restart", func() {
			By("Creating deployment")
			f := path.Join(examplesDir, "quarks-restart/deployment.yaml")
			err := testing.Create(namespace, f)
			Expect(err).ToNot(HaveOccurred())

			var podName string
			Eventually(func() string {
				podName = getPodName(selector)
				return podName
			}).ShouldNot(BeEmpty())
			waitReady("pod/" + podName)

			By("Updating secret")
			updateSecret()

			err = kubectl.WaitForPodDelete(namespace, podName)
			Expect(err).ToNot(HaveOccurred(), "waiting for old pod to terminate")

			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", selector)
			Expect(err).ToNot(HaveOccurred())

			podName = getPodName(selector)
			By("checking restart annotation of the pod")
			err = kubectl.WaitForData(
				namespace, "pod", podName,
				`jsonpath="{.metadata.annotations}"`,
				quarksrestart.RestartKey,
			)
			Expect(err).ToNot(HaveOccurred(), "waiting for restart annotation on the pod")
		})

		It("the statefulset should restart", func() {
			By("Creating statefulset")
			f := path.Join(examplesDir, "quarks-restart/statefulset.yaml")
			err := testing.Create(namespace, f)
			Expect(err).ToNot(HaveOccurred())

			podName := getPodName(selector)
			waitReady("pod/" + podName)

			By("Updating secret")
			updateSecret()

			By("checking restart annotation of the pod")
			err = kubectl.WaitForData(
				namespace, "pod", podName,
				`jsonpath="{.metadata.annotations}"`,
				quarksrestart.RestartKey,
			)
			Expect(err).ToNot(HaveOccurred(), "waiting for restart annotation on the pod")
		})
	})
})
