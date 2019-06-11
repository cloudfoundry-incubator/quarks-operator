package names_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

var _ = Describe("Names", func() {

	type test struct {
		expectation string
		result      string
	}

	Context("Sanitize", func() {
		// according to docs/naming.md
		long31 := "a123456789012345678901234567890"
		long63 := long31 + "b123456789012345678901234567890c"
		tests := []test{
			{expectation: "AB1", result: "ab1"},
			{expectation: "ab1", result: "ab1"},
			{expectation: "1bc", result: "1bc"},
			{expectation: "a-b", result: "a-b"},
			{expectation: "a_b", result: "a-b"},
			{expectation: "a_b_123", result: "a-b-123"},
			{expectation: "-abc", result: "abc"},
			{expectation: "abc-", result: "abc"},
			{expectation: "_abc_", result: "abc"},
			{expectation: "-abc-", result: "abc"},
			{expectation: "abcü.123:4", result: "abc1234"},
			{expectation: long63, result: long63},
			{expectation: long63 + "0", result: long31 + "f61acdbce0e8ea6e4912f53bde4de866"},
		}
		It("produces valid k8s names", func() {
			for _, t := range tests {
				Expect(names.Sanitize(t.expectation)).To(Equal(t.result), fmt.Sprintf("%#v", t))
			}
		})
	})
})
