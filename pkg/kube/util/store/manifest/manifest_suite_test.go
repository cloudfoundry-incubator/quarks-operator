package manifeststore_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestManifestPersistence(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, `Versioning and persistence for desired manifests Suite`)
}
