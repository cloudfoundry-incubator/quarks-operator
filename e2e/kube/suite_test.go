package kube_test

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"
)

const examplesDir = "../../docs/examples/"

var (
	nsIndex           int
	teardowns         []e2ehelper.TearDownFunc
	namespace         string
	operatorNamespace string
	kubectl           *cmdHelper.Kubectl
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
	var teardown e2ehelper.TearDownFunc

	dir, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())

	chartPath := fmt.Sprintf("%s%s", dir, "/../../helm/cf-operator")

	namespace, operatorNamespace, teardown, err = e2ehelper.CreateNamespace()
	Expect(err).ToNot(HaveOccurred())
	teardowns = append(teardowns, teardown)

	teardown, err = e2ehelper.InstallChart(chartPath, operatorNamespace,
		"--set", fmt.Sprintf("global.singleNamespace.name=%s", namespace),
		"--set", fmt.Sprintf("global.monitoredID=%s", namespace),
		"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", namespace),
	)
	Expect(err).ToNot(HaveOccurred())
	// prepend helm clean up
	teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)
})

var _ = AfterEach(func() {
	e2ehelper.TearDownAll(teardowns)
})

func podWait(name string) {
	err := kubectl.Wait(namespace, "ready", name, kubectl.PollTimeout)
	Expect(err).ToNot(HaveOccurred())
}
