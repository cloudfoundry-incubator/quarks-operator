package kube_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"
)

var _ = Describe("Deploying in multiple namespace", func() {
	var newNamespace string

	BeforeEach(func() {
		var err error
		var tearDowns []e2ehelper.TearDownFunc

		newNamespace, tearDowns, err = e2ehelper.CreateMonitoredNamespaceFromExistingRole(namespace)
		Expect(err).ToNot(HaveOccurred())
		teardowns = append(teardowns, tearDowns...)
	})

	Context("when creating a bosh deployment in two monitored namespaces", func() {
		It("creates a service for quarks-gora in both", func() {
			Expect(newNamespace != namespace).To(BeTrue()) // Sanity check - we shouldn't run the test on the same original namespace

			applyNamespace(namespace, "bosh-deployment/quarks-gora.yaml")
			waitReadyNamespace(namespace, "pod/quarks-gora-0")
			waitReadyNamespace(namespace, "pod/quarks-gora-1")
			err := kubectl.WaitForService(namespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForService(namespace, "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())

			applyNamespace(newNamespace, "bosh-deployment/quarks-gora.yaml")
			waitReadyNamespace(newNamespace, "pod/quarks-gora-0")
			waitReadyNamespace(newNamespace, "pod/quarks-gora-1")
			err = kubectl.WaitForService(newNamespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForService(newNamespace, "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("when updating bosh deployments in two monitored namespaces", func() {

		scale := func(namespace, i string) {
			cfgmap, err := kubectl.GetConfigMap(namespace, "ops-scale")
			Expect(err).ToNot(HaveOccurred())
			Expect(cfgmap.Metadata.Name).To(Equal("ops-scale"))
			cfgmap.Data["ops"] = `- type: replace
  path: /instance_groups/name=quarks-gora?/instances
  value: ` + i

			err = kubectl.ApplyYAML(namespace, "configmap", &cfgmap)
			Expect(err).ToNot(HaveOccurred())
		}

		It("scales quarks-gora in each namespace respectively", func() {

			applyNamespace(namespace, "bosh-deployment/quarks-gora.yaml")
			waitReadyNamespace(namespace, "pod/quarks-gora-0")
			waitReadyNamespace(namespace, "pod/quarks-gora-1")
			err := kubectl.WaitForService(namespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForService(namespace, "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())

			applyNamespace(newNamespace, "bosh-deployment/quarks-gora.yaml")
			waitReadyNamespace(newNamespace, "pod/quarks-gora-0")
			waitReadyNamespace(newNamespace, "pod/quarks-gora-1")
			err = kubectl.WaitForService(newNamespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForService(newNamespace, "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())

			scale(namespace, "3")
			waitReadyNamespace(namespace, "pod/quarks-gora-2")
			err = kubectl.WaitForService(namespace, "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())

			scale(newNamespace, "4")
			waitReadyNamespace(newNamespace, "pod/quarks-gora-3")
			err = kubectl.WaitForService(newNamespace, "quarks-gora-3")
			Expect(err).ToNot(HaveOccurred())

			e, err := kubectl.ServiceExists(namespace, "quarks-gora-3")
			Expect(err).To(HaveOccurred())
			Expect(e).ToNot(BeTrue())
		})
	})

})
