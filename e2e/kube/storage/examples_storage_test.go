package storage_kube_test

import (
	"io/ioutil"
	"os"

	"sigs.k8s.io/yaml"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

// AddTestStorageClassToVolumeClaimTemplates adds storage class to the example and returns the new file temporary path
func AddTestStorageClassToVolumeClaimTemplates(filePath string, class string) (string, error) {
	qStatefulSet := qstsv1a1.QuarksStatefulSet{}
	qStatefulSetBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "Reading file %s failed.", filePath)
	}
	err = yaml.Unmarshal(qStatefulSetBytes, &qStatefulSet)
	if err != nil {
		return "", errors.Wrapf(err, "Unmarshalling quarksStatefulSet from file %s failed.", filePath)
	}

	if qStatefulSet.Spec.Template.Spec.VolumeClaimTemplates != nil {
		volumeClaimTemplates := qStatefulSet.Spec.Template.Spec.VolumeClaimTemplates
		for volumeClaimTemplateIndex := range volumeClaimTemplates {
			volumeClaimTemplates[volumeClaimTemplateIndex].Spec.StorageClassName = pointers.String(class)
		}
		qStatefulSet.Spec.Template.Spec.VolumeClaimTemplates = volumeClaimTemplates
	} else {
		return "", errors.Errorf("No volumeclaimtemplates present in the %s yaml", filePath)
	}

	qStatefulSetBytes, err = yaml.Marshal(&qStatefulSet)
	if err != nil {
		return "", errors.Wrapf(err, "Marshing quarksStatefulSet %s failed", qStatefulSet.GetName())
	}

	tmpFilePath := "/tmp/example.yaml"

	err = ioutil.WriteFile(tmpFilePath, qStatefulSetBytes, 0644)
	if err != nil {
		return "", errors.Wrapf(err, "Writing quarksStatefulSet %s to file %s failed.", qStatefulSet.GetName(), tmpFilePath)
	}

	return tmpFilePath, nil
}

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

			It("quarks-statefulset pvc example must work", func() {
				yamlFilePath := examplesDir + "quarks-statefulset/qstatefulset_pvcs.yaml"

				// Adding storageclass to volumeclaimtemplates
				class, ok := os.LookupEnv("OPERATOR_TEST_STORAGE_CLASS")
				Expect(ok).To(Equal(true))

				exampleTmpFilePath, err := AddTestStorageClassToVolumeClaimTemplates(yamlFilePath, class)
				Expect(err).ToNot(HaveOccurred())

				By("Creating quarksStatefulSet pvcs")
				err = cmdHelper.Create(namespace, exampleTmpFilePath)
				Expect(err).ToNot(HaveOccurred())

				By("Checking for pods")
				err = kubectlHelper.Wait(namespace, "ready", "pod/example-quarks-statefulset-0", kubectlHelper.PollTimeout)
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.Wait(namespace, "ready", "pod/example-quarks-statefulset-1", kubectlHelper.PollTimeout)
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.WaitForPVC(namespace, "pvc-example-quarks-statefulset-0")
				Expect(err).ToNot(HaveOccurred())

				err = kubectlHelper.WaitForPVC(namespace, "pvc-example-quarks-statefulset-1")
				Expect(err).ToNot(HaveOccurred())

				// Delete the temporary file
				err = os.Remove(exampleTmpFilePath)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
