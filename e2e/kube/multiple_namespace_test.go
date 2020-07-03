package kube_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"
)

var _ = Describe("Deploying in multiple namespace", func() {
	kubectl = cmdHelper.NewKubectl()
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
})
