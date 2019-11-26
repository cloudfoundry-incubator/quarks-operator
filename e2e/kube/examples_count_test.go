package kube_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Examples Directory Files", func() {
	It("Test cases must be written for all example use cases in docs", func() {
		countFile := 0
		err := filepath.Walk(examplesDir, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				countFile = countFile + 1
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		// If this testcase fails that means a test case is missing for an example in the docs folder
		Expect(countFile).To(Equal(21))
	})
})
