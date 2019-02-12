package manifest_test

import (
	"fmt"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Manifest", func() {
	var (
		job Job
	)

	BeforeEach(func() {
		job = Job{Name: "redis-server",
			Release:    "redis",
			Properties: map[string]interface{}{"port": 3606},
		}
	})

	Describe("converting to Yaml from Schema", func() {
		It("job schema should match its yaml", func() {
			y, err := yaml.Marshal(job)
			fmt.Println(string(y))
			Expect(string(y)).To(Equal("name: redis-server\n" +
				"release: redis\n" +
				"consumes: {}\n" +
				"provides: {}\n" +
				"properties:\n" +
				"  port: 3606\n"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
