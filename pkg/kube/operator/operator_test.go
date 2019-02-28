package operator_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("operator", func() {
	Describe("GetOperatorDockerImage", func() {
		It("returns the location of the docker image", func() {
			version.Version = "1.2.3"
			operator.DockerOrganization = "foo"
			operator.DockerRepository = "bar"

			Expect(operator.GetOperatorDockerImage()).To(Equal("foo/bar:1.2.3"))
		})
	})
})
