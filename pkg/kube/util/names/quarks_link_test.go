package names_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

var _ = Describe("QuarksLink related names", func() {
	Context("link type and name strings", func() {
		It("should return a simple link type and link name string", func() {
			Expect(names.QuarksLinkSecretKey("type", "name")).To(Equal("type-name"))
		})
	})

	Context("link secret name strings", func() {
		It("should return a string to be used as a secret name for links without a suffix", func() {
			Expect(names.QuarksLinkSecretName("deploymentname")).To(Equal("link-deploymentname"))
		})

		It("should return a string to be used as a secret name for links with one suffix", func() {
			Expect(names.QuarksLinkSecretName("deploymentname", "suffix")).To(Equal("link-deploymentname-suffix"))
		})

		It("should return a string to be used as a secret name for links with multiple suffixes", func() {
			Expect(names.QuarksLinkSecretName("deploymentname", "one", "two")).To(Equal("link-deploymentname-one-two"))
		})
	})
})
