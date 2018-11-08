package boshdeployment_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestBoshdeployment(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Boshdeployment Suite")
}
