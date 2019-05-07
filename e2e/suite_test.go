package e2e_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestE2E(t *testing.T) {
	SetDefaultEventuallyTimeout(10 * time.Second)
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var (
	cliPath string
)

var _ = BeforeSuite(func() {
	var err error
	cliPath, err = gexec.Build("code.cloudfoundry.org/cf-operator/cmd/cf-operator")
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	gexec.KillAndWait()
	gexec.CleanupBuildArtifacts()
})
