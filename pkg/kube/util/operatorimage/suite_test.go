package operatorimage_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestOperatorSet(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Operator Image Suite")
}
