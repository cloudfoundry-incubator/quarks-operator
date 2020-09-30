package upgrade_suite_test

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	teardowns   []e2ehelper.TearDownFunc
	examplesDir string
	kubectl     *cmdHelper.Kubectl
)

func TestUpgrades(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "E2E Kube Upgrade Suite")
}

var _ = BeforeSuite(func() {
	kubectl = cmdHelper.NewKubectl()
	SetDefaultEventuallyTimeout(kubectl.PollTimeout)
	SetDefaultEventuallyPollingInterval(500 * time.Millisecond)

	_, err := e2ehelper.AddHelmRepo("quarks", "https://cloudfoundry-incubator.github.io/quarks-helm")
	Expect(err).ToNot(HaveOccurred(), "cannot add helm repo")

	dir, _ := os.Getwd()
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

func waitReady(ns, name string) {
	err := kubectl.Wait(ns, "ready", name, kubectl.PollTimeout)
	Expect(err).ToNot(HaveOccurred(), "waiting for resource: ", name)
}

func apply(ns, p string) {
	yamlPath := path.Join(examplesDir, p)
	err := cmdHelper.Apply(ns, yamlPath)
	Expect(err).ToNot(HaveOccurred())
}

func scale(namespace, i string) {
	cfg, err := kubectl.GetConfigMap(namespace, "ops-scale")
	Expect(err).ToNot(HaveOccurred())
	cfg.Data["ops"] = `- type: replace
  path: /instance_groups/name=quarks-gora?/instances
  value: ` + i

	err = kubectl.ApplyYAML(namespace, "configmap", &cfg)
	Expect(err).ToNot(HaveOccurred())
}

func checkEntanglement(namespace, podName, cmd, expect string) error {
	return kubectl.RunCommandWithCheckString(
		namespace, podName,
		cmd,
		expect,
	)
}

func getPodName(namespace, selector string) string {
	podNames, err := kubectl.GetPodNames(namespace, selector)
	Expect(err).ToNot(HaveOccurred())
	Expect(podNames[0]).ToNot(Equal(""))
	return podNames[0]
}
