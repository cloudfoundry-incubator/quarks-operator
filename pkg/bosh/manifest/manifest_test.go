package manifest_test

import (
	"fmt"
	"log"

	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
)

var _ = Describe("Manifest", func() {
	var (
		addOnStemcell AddOnStemcell
		job           Job
	)

	BeforeEach(func() {
		addOnStemcell = AddOnStemcell{OS: "Linux"}
		job = Job{Name: "redis-server",
			Release:    "redis",
			Properties: map[string]interface{}{"port": 3606},
		}
	})

	Describe("converting to Yaml from Schema", func() {
		It("addonstemcell schema should match its yaml", func() {
			y, err := yaml.Marshal(addOnStemcell)
			Expect(string(y)).To(Equal("os: Linux\n"))
			if err != nil {
				log.Fatalf("error: %v", err)
			}
		})

		It("job schema should match its yaml", func() {
			y, err := yaml.Marshal(job)
			fmt.Println(string(y))
			Expect(string(y)).To(Equal("name: redis-server\n" +
				"release: redis\n" +
				"consumes: {}\n" +
				"provides: {}\n" +
				"properties:\n" +
				"  port: 3606\n"))
			if err != nil {
				log.Fatalf("error: %v", err)
			}
		})

	})
})
