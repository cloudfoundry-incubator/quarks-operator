package upgrade_suite_test

import (
	"fmt"
	"os"
	"path"
	"testing"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	teardowns         []e2ehelper.TearDownFunc
	namespace         string
	operatorNamespace string
	examplesDir       string
	kubectl           *cmdHelper.Kubectl
)

func TestUpgrades(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "E2E Kube Upgrade Suite")
}

var _ = BeforeSuite(func() {
	dir, _ := os.Getwd()
	kubectl = cmdHelper.NewKubectl()
	examplesDir = fmt.Sprintf("%s%s", dir, "/../../../docs/examples/")
})

var _ = AfterEach(func() {
	err := e2ehelper.TearDownAll(teardowns)
	if err != nil {
		fmt.Printf("Failures while cleaning up test environment:\n %v", err)
	}
	teardowns = []e2ehelper.TearDownFunc{}
})

var _ = AfterSuite(func() {
	err := e2ehelper.TearDownAll(teardowns)
	if err != nil {
		fmt.Printf("Failures while cleaning up test environment:\n %v", err)
	}
})

func waitReady(name string) {
	waitReadyNamespace(namespace, name)
}

func waitReadyNamespace(ns, name string) {
	err := kubectl.Wait(ns, "ready", name, kubectl.PollTimeout)
	Expect(err).ToNot(HaveOccurred(), "waiting for resource: ", name)
}

func applyNamespace(ns, p string) {
	yamlPath := path.Join(examplesDir, p)
	err := cmdHelper.Apply(ns, yamlPath)
	Expect(err).ToNot(HaveOccurred())
}
