package quarkssecret_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestQuarksSecret(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "QuarksSecret Suite")
}
