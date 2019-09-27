package operator_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("operator", func() {
	Describe("GetOperatorDockerImage", func() {
		It("returns the location of the docker image", func() {
			err := converter.SetupOperatorDockerImage("foo", "bar", "1.2.3")
			Expect(err).ToNot(HaveOccurred())
			Expect(converter.GetOperatorDockerImage()).To(Equal("foo/bar:1.2.3"))
		})
	})
})
