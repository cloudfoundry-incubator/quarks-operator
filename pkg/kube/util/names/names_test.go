package names_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

var _ = Describe("Names", func() {
	type test struct {
		arg1   string
		result string
		n      int
	}
	long31 := "a123456789012345678901234567890"
	long63 := long31 + "b123456789012345678901234567890c"

	Context("JobName", func() {
		tests := []test{
			{arg1: "ab1", result: "ab1", n: 20},
			{arg1: "a-b1", result: "a-b1-", n: 21},
			{arg1: long31, result: "a123456789012345678901234567890-", n: 48},
			{arg1: long63, result: long63[:39] + "-", n: 56},
		}

		It("produces valid k8s job names", func() {
			for _, t := range tests {
				r, err := names.JobName(t.arg1)
				Expect(err).ToNot(HaveOccurred())
				Expect(r).To(ContainSubstring(t.result), fmt.Sprintf("%#v", t))
				Expect(r).To(HaveLen(t.n), fmt.Sprintf("%#v", t))
			}
		})
	})

	Context("Sanitize", func() {
		// according to docs/naming.md
		tests := []test{
			{arg1: "AB1", result: "ab1"},
			{arg1: "ab1", result: "ab1"},
			{arg1: "1bc", result: "1bc"},
			{arg1: "a-b", result: "a-b"},
			{arg1: "a_b", result: "a-b"},
			{arg1: "a_b_123", result: "a-b-123"},
			{arg1: "-abc", result: "abc"},
			{arg1: "abc-", result: "abc"},
			{arg1: "_abc_", result: "abc"},
			{arg1: "-abc-", result: "abc"},
			{arg1: "abcü.123:4", result: "abc1234"},
			{arg1: long63, result: long63},
			{arg1: long63 + "0", result: long31 + "f61acdbce0e8ea6e4912f53bde4de866"},
		}

		It("produces valid k8s names", func() {
			for _, t := range tests {
				Expect(names.Sanitize(t.arg1)).To(Equal(t.result), fmt.Sprintf("%#v", t))
			}
		})
	})
})
