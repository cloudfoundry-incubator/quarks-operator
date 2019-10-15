package storage_kube_test

import (
	"io/ioutil"
	"os"

	"github.com/pkg/errors"

	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

// AddTestStorageClassToVolumeClaimTemplates adds storage class to the example and returns the new file temporary path
func AddTestStorageClassToVolumeClaimTemplates(filePath string, class string) (string, error) {
	extendedStatefulSet := essv1.ExtendedStatefulSet{}
	extendedStatefulSetBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "Reading file %s failed.", filePath)
	}
	err = yaml.Unmarshal(extendedStatefulSetBytes, &extendedStatefulSet)
	if err != nil {
		return "", errors.Wrapf(err, "Unmarshalling extendedstatefulset from file %s failed.", filePath)
	}

	if extendedStatefulSet.Spec.Template.Spec.VolumeClaimTemplates != nil {
		volumeClaimTemplates := extendedStatefulSet.Spec.Template.Spec.VolumeClaimTemplates
		for volumeClaimTemplateIndex := range volumeClaimTemplates {
			volumeClaimTemplates[volumeClaimTemplateIndex].Spec.StorageClassName = pointers.String(class)
		}
		extendedStatefulSet.Spec.Template.Spec.VolumeClaimTemplates = volumeClaimTemplates
	} else {
		return "", errors.Errorf("No volumeclaimtemplates present in the %s yaml", filePath)
	}

	extendedStatefulSetBytes, err = yaml.Marshal(&extendedStatefulSet)
	if err != nil {
		return "", errors.Wrapf(err, "Marshing extendedstatfulset %s failed", extendedStatefulSet.GetName())
	}

	tmpFilePath := "/tmp/example.yaml"

	err = ioutil.WriteFile(tmpFilePath, extendedStatefulSetBytes, 0644)
	if err != nil {
		return "", errors.Wrapf(err, "Writing extendedstatefulset %s to file %s failed.", extendedStatefulSet.GetName(), tmpFilePath)
	}

	return tmpFilePath, nil
}

var _ = Describe("Examples", func() {

	Describe("when storage related examples are specified in the docs", func() {

		var (
			kubectlHelper *cmdHelper.Kubectl
		)
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
				err := cmdHelper.CreateSecretFromLiteral(namespace, "nats-deployment.var-operator-test-storage-class", literalValues)
				Expect(err).ToNot(HaveOccurred())

				By("Creating bosh deployment")
				err = cmdHelper.Create(namespace, yamlFilePath)
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pods")
				err = kubectlHelper.Wait(namespace, "ready", "pod/nats-deployment-nats-v1-0", kubectlHelper.PollTimeout)
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.Wait(namespace, "ready", "pod/nats-deployment-nats-v1-1", kubectlHelper.PollTimeout)
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

				exampleTmpFilePath, err := AddTestStorageClassToVolumeClaimTemplates(yamlFilePath, class)
				Expect(err).ToNot(HaveOccurred())

				By("Creating exstatefulset pvcs")
				err = cmdHelper.Create(namespace, exampleTmpFilePath)
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pods")
				err = kubectlHelper.Wait(namespace, "ready", "pod/example-extendedstatefulset-v1-0", kubectlHelper.PollTimeout)
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.Wait(namespace, "ready", "pod/example-extendedstatefulset-v1-1", kubectlHelper.PollTimeout)
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
