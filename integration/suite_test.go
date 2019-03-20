package integration_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
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
		time.Sleep(3 * time.Second)
		defer stopOperator()
	}
})
