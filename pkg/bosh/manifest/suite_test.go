package manifest_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const assetPath = "../../../testing/assets"

func TestManifest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "BOSH Manifest Suite")
}
