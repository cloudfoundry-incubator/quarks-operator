package kube_test

import (
	"fmt"
	"os/exec"
	"testing"

	"code.cloudfoundry.org/cf-operator/e2e/kube/e2ehelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
)

var (
	nsIndex    int
	nsTeardown environment.TearDownFunc
	namespace  string
)

func FailAndCollectDebugInfo(description string, callerSkip ...int) {
	fmt.Println("Collecting debug information...")
	out, err := exec.Command("../../testing/dump_env.sh", namespace).CombinedOutput()
	if err != nil {
		fmt.Println("Failed to run the `dump_env.sh` script", err)
	}
	fmt.Println(string(out))

	Fail(description, callerSkip...)
}

func TestE2EKube(t *testing.T) {
	nsIndex = 0

	RegisterFailHandler(FailAndCollectDebugInfo)
	RunSpecs(t, "E2E Kube Suite")
}

var _ = BeforeEach(func() {
	var err error
	nsTeardown, err = e2ehelper.SetUpEnvironment()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	if nsTeardown != nil {
		nsTeardown()
	}
})
