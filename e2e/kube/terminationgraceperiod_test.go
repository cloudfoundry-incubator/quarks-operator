package kube_test

import (
	"path"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BOSHDeployment", func() {
	When("specifying terminationGracePeriodSeconds in bosh.env", func() {
		BeforeEach(func() {
			By("Creating bdpl")
			f := path.Join(examplesDir, "bosh-deployment/quarks-gora-termination.yaml")
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
				requestedGrace, err := cmdHelper.GetData(namespace, "pods", pod, "jsonpath={.spec.terminationGracePeriodSeconds}")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(requestedGrace)).To(Equal("900"))
			}
		})
	})
})
