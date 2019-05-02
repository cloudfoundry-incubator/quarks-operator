package kube_test

import (
	"time"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Kube Suite")
}

var (
	cliPath      string
	env          *environment.Environment
	stopOperator environment.StopFunc
)

var _ = BeforeSuite(func() {
	var err error
	cliPath, err = gexec.Build("code.cloudfoundry.org/cf-operator/cmd/cf-operator")
	Expect(err).ToNot(HaveOccurred())

	env = environment.NewEnvironment()
	stopOperator, err = env.Setup()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if stopOperator != nil {
		time.Sleep(3 * time.Second)
		defer stopOperator()
	}

	gexec.KillAndWait()
	gexec.CleanupBuildArtifacts()
})
