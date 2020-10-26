package storage_kube_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

var _ = Describe("Examples", func() {
	Describe("when storage related examples are specified in the docs", func() {
		var kubectlHelper *cmdHelper.Kubectl
		const examplesDir = "../../../docs/examples/"

		BeforeEach(func() {
			kubectlHelper = cmdHelper.NewKubectl()
		})

		Context("all storage related examples with storage must be working", func() {
			It("bosh-deployment with a persistent disk example must work", func() {
				yamlFilePath := examplesDir + "bosh-deployment/boshdeployment-with-persistent-disk.yaml"

				By("Creating a secret for implicit variable")
				class, ok := os.LookupEnv("OPERATOR_TEST_STORAGE_CLASS")
				Expect(ok).To(Equal(true))

				literalValues := map[string]string{
					"value": class,
				}
				err := cmdHelper.CreateSecretFromLiteral(namespace, "var-operator-test-storage-class", literalValues)
				Expect(err).ToNot(HaveOccurred())

				By("Creating bosh deployment")
				err = cmdHelper.Create(namespace, yamlFilePath)
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pods")
				err = kubectlHelper.Wait(namespace, "ready", "pod/nats-0", kubectlHelper.PollTimeout)
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.Wait(namespace, "ready", "pod/nats-1", kubectlHelper.PollTimeout)
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pvcs")
				err = kubectlHelper.WaitForPVC(namespace, "nats-pvc-nats-0")
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.WaitForPVC(namespace, "nats-pvc-nats-1")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
