package quarksstatefulset_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestQuarksStatefulSet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QuarksStatefulSet Suite")
}
