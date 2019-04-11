package versionedsecretstore_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestVersionedSecretStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Versioned secret store Suite")
}
