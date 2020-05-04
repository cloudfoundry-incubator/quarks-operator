package waitservice_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestQuarksWaitService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WaitService Suite")
}
