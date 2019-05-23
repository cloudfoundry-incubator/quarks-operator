package integration_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
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
	nsTeardown   environment.TearDownFunc
)

var _ = BeforeSuite(func() {
	env = environment.NewEnvironment()

	var err error
	nsTeardown, stopOperator, err = env.Setup(int32(config.GinkgoConfig.ParallelNode))
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	if stopOperator != nil {
		time.Sleep(3 * time.Second)
		defer stopOperator()
	}
	if nsTeardown != nil {
		nsTeardown()
	}
})
