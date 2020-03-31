package names_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

var _ = Describe("Names", func() {
	Context("InstanceGroupSecretName", func() {
		type test struct {
			arg3 string
			arg4 string
			name string
		}
		tests := []test{
			{
				arg3: "ig-Name",
				arg4: "", // "0.1",
				name: "ig-resolved.ig-name",
			},
			{
				arg3: "ig_Name",
				arg4: "",
				name: "ig-resolved.ig-name",
			},
			{
				arg3: "igname12345678901234567890ABC" + text225,
				arg4: "",
				name: "ig-resolved.igname12345678901234567890abcthis-is-w" + text171[:170] + "-d1a31c527eb0ad85b571d5028e28573c",
			},
		}

		It("produces valid k8s secret names", func() {
			for _, t := range tests {
				r := names.InstanceGroupSecretName(t.arg3, t.arg4)
				Expect(r).To(Equal(t.name), fmt.Sprintf("%#v", t))
			}
		})
	})
})
