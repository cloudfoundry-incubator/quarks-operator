package integration_test

import (
	"fmt"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
)

func FailAndCollectDebugInfo(description string, callerSkip ...int) {
	fmt.Println("Collecting debug information...")
	out, err := exec.Command("../testing/dump_env.sh", env.Namespace).CombinedOutput()
	if err != nil {
		fmt.Println("Failed to run the `dump_env.sh` script", err)
	}
	fmt.Println(string(out))

	skip := 0
	if len(callerSkip) > 0 {
		skip = callerSkip[0]
	}
	Fail(description, skip+1)
}

func TestIntegration(t *testing.T) {
	RegisterFailHandler(FailAndCollectDebugInfo)
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
	env.RemoveWebhookCache(int32(config.GinkgoConfig.ParallelNode))
	if stopOperator != nil {
		time.Sleep(3 * time.Second)
		defer stopOperator()
	}
	if nsTeardown != nil {
		nsTeardown()
	}
})
