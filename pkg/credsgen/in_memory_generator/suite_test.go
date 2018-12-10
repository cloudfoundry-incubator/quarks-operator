package inmemorygenerator_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestInMemoryGenerator(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "InMemoryGenerator Suite")
}
