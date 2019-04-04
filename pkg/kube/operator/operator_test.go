package operator_test

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("operator", func() {
	Describe("GetOperatorDockerImage", func() {
		It("returns the location of the docker image", func() {
			manifest.DockerImageTag = "1.2.3"
			manifest.DockerImageOrganization = "foo"
			manifest.DockerImageRepository = "bar"

			Expect(manifest.GetOperatorDockerImage()).To(Equal("foo/bar:1.2.3"))
		})
	})
})
