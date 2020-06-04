package kube_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

var _ = Describe("K8s native resources provide BOSH links to a BOSH deployment", func() {
	kubectl = cmdHelper.NewKubectl()

	Context("when a service is used", func() {
		It("uses kube native link", func() {
			err := apply("quarks-link/native-to-bosh/link-pod.yaml")
			Expect(err).ToNot(HaveOccurred())
			err = apply("quarks-link/native-to-bosh/link-secret.yaml")
			Expect(err).ToNot(HaveOccurred())
			err = apply("quarks-link/native-to-bosh/link-service.yaml")
			Expect(err).ToNot(HaveOccurred())

			err = apply("quarks-link/native-to-bosh/boshdeployment.yaml")
			Expect(err).ToNot(HaveOccurred())

			podWait("pod/draining-ig-0")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
