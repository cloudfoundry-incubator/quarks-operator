package quarkslink_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestQuarksLink(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QuarksLink Suite")
}
