package containerrun_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestContainerrun(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "containerrun cmd Suite")
}
