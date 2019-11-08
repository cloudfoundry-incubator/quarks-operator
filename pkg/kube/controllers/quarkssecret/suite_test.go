package quarkssecret_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestExtendedSecret(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QuarksSecret Suite")
}
