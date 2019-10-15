package kube_test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"
)

const examplesDir = "../../docs/examples/"

var (
	nsIndex   int
	teardown  e2ehelper.TearDownFunc
	namespace string
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

	dir, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())

	chartPath := fmt.Sprintf("%s%s", dir, "/../../helm/cf-operator")
	namespace, teardown, err = e2ehelper.SetUpEnvironment(chartPath)
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	if teardown != nil {
		teardown()
	}
})
