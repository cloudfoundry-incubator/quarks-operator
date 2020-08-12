package kube_test

import (
	"path"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BOSHDeployment", func() {
	When("specifying cpu/memory resources requests and limits in the quarks.bpm config", func() {
		kubectl := cmdHelper.NewKubectl()

		BeforeEach(func() {
			By("Creating bdpl")
			f := path.Join(examplesDir, "bosh-deployment/quarks-gora-cpu-mem.yaml")
			err := cmdHelper.Create(namespace, f)
			Expect(err).ToNot(HaveOccurred())

			By("Checking for pods")
			err = kubectl.Wait(namespace, "ready", "pod/quarks-gora-0", kubectl.PollTimeout)
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.Wait(namespace, "ready", "pod/quarks-gora-1", kubectl.PollTimeout)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should create containers with the requested resources/limits", func() {
			for _, pod := range []string{"quarks-gora-0", "quarks-gora-1"} {
				requestedMemory, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.spec.containers[0].resources.requests.memory}")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(requestedMemory)).To(Equal("128Mi"))
				requestedCPU, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.spec.containers[0].resources.requests.cpu}")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(requestedCPU)).To(Equal("2m"))
				limitMemory, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.spec.containers[0].resources.limits.memory}")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(limitMemory)).To(Equal("2Gi"))
				limitCPU, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.spec.containers[0].resources.limits.cpu}")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(limitCPU)).To(Equal("4"))
			}
		})
	})
})
