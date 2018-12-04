package integration_test

import (
	"testing"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var (
	env          *environment.Environment
	stopOperator environment.StopFunc
)

var _ = BeforeSuite(func() {
	env = environment.NewEnvironment()

	var err error
	stopOperator, err = env.Setup()
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if stopOperator != nil {
		defer stopOperator()
	}
})
