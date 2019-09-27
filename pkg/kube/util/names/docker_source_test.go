package names_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

var _ = Describe("DockerSource", func() {
	type test struct {
		org    string
		repo   string
		tag    string
		result string
	}

	Context("GetName", func() {
		Context("when name is valid", func() {
			tests := []test{
				{org: "cfcontainerization", repo: "cf-operator", tag: "0.1-dev",
					result: "cfcontainerization/cf-operator:0.1-dev"},
				{org: "cfcontainerization", repo: "cf-operator",
					result: "cfcontainerization/cf-operator"},
				{org: "", repo: "cf-operator", tag: "0.1-dev",
					result: "cf-operator:0.1-dev"},
				{org: "", repo: "cf-operator",
					result: "cf-operator"},
			}

			It("produces valid docker image sources", func() {
				for _, t := range tests {
					name, err := GetDockerSourceName(t.org, t.repo, t.tag)
					Expect(err).ToNot(HaveOccurred())
					Expect(name).To(Equal(t.result), fmt.Sprintf("%#v", t))
				}
			})
		})

		Context("when name is invalid", func() {
			tests := []test{
				{org: "", repo: "", tag: "0.1-dev"},
				{org: "fake-org", repo: "", tag: "0.1-dev"},
				{org: "fake-org", repo: ""},
			}

			It("returns an error", func() {
				for _, t := range tests {
					_, err := GetDockerSourceName(t.org, t.repo, t.tag)
					Expect(err).To(HaveOccurred())
				}
			})
		})
	})
})
