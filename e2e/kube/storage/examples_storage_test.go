package storage_kube_test

import (
	"code.cloudfoundry.org/cf-operator/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Examples", func() {

	Describe("when examples are specified in the docs", func() {

		const examplesDir = "../../../docs/examples/"

		Context("all examples with storage must be working", func() {

			It("bosh-deployment with a persistent disk example must work", func() {
				yamlFilePath := examplesDir + "bosh-deployment/boshdeployment-with-persistent-disk.yaml"

				By("Creating bosh deployment")
				kubectlHelper := testing.NewKubectl()
				err := kubectlHelper.Create(namespace, yamlFilePath)
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pods")
				err = kubectlHelper.Wait(namespace, "ready", "pod/nats-deployment-nats-v1-0")
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.Wait(namespace, "ready", "pod/nats-deployment-nats-v1-1")
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pvc")
				err = kubectlHelper.WaitForPVC(namespace, "nats-deployment-nats-pvc")
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
