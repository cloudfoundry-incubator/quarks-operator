package storage_kube_test

import (
	"os"

	"code.cloudfoundry.org/cf-operator/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Examples", func() {

	Describe("when storage related examples are specified in the docs", func() {

		var (
			kubectlHelper *testing.Kubectl
		)
		const examplesDir = "../../../docs/examples/"

		BeforeEach(func() {
			kubectlHelper = testing.NewKubectl()
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
				err := testing.CreateSecretFromLiteral(namespace, "nats-deployment.var-implicit-operator-test-storage-class", literalValues)
				Expect(err).ToNot(HaveOccurred())

				By("Creating bosh deployment")
				err = testing.Create(namespace, yamlFilePath)
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

			It("extended-statefulset pvc example must work", func() {
				yamlFilePath := examplesDir + "extended-statefulset/exstatefulset_pvcs.yaml"

				// Adding storageclass to volumeclaimtemplates
				class, ok := os.LookupEnv("OPERATOR_TEST_STORAGE_CLASS")
				Expect(ok).To(Equal(true))

				exampleTmpFilePath, err := testing.AddTestStorageClassToVolumeClaimTemplates(yamlFilePath, class)
				Expect(err).ToNot(HaveOccurred())

				By("Creating exstatefulset pvcs")
				err = testing.Create(namespace, exampleTmpFilePath)
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pods")
				err = kubectlHelper.Wait(namespace, "ready", "pod/example-extendedstatefulset-v1-0")
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.Wait(namespace, "ready", "pod/example-extendedstatefulset-v1-1")
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.WaitForPVC(namespace, "pvc-volume-management-example-extendedstatefulset-0")
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.WaitForPVC(namespace, "pvc-volume-management-example-extendedstatefulset-1")
				Expect(err).ToNot(HaveOccurred())

				// Delete the temporary file
				err = os.Remove(exampleTmpFilePath)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
