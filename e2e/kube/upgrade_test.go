package kube_test

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"
)

//
var _ = Describe("Quarks-operator Upgrade test", func() {
	kubectl = cmdHelper.NewKubectl()
	//	var newNamespace string

	JustBeforeEach(func() {
		// Delete previous deployment and reset teardowns
		err := e2ehelper.TearDownAll(teardowns)
		if err != nil {
			fmt.Printf("Failures while cleaning up test environment:\n %v", err)
		}

		teardowns = []e2ehelper.TearDownFunc{}
		// Delete also CRDs
		// Note: if this turns out to be very flaky, consider to move this test
		// to a separate suite.
		err = kubectl.Delete("crds", "boshdeployments.quarks.cloudfoundry.org")
		Expect(err).ToNot(HaveOccurred())
		err = kubectl.Delete("crds", "quarksjobs.quarks.cloudfoundry.org")
		Expect(err).ToNot(HaveOccurred())
		err = kubectl.Delete("crds", "quarkssecrets.quarks.cloudfoundry.org")
		Expect(err).ToNot(HaveOccurred())
		err = kubectl.Delete("crds", "quarksstatefulsets.quarks.cloudfoundry.org")
		Expect(err).ToNot(HaveOccurred())

		teardown, err := e2ehelper.AddHelmRepo("quarks", "https://cloudfoundry-incubator.github.io/quarks-helm")
		Expect(err).ToNot(HaveOccurred())
		teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)
	})

	Context("latest available chart", func() {
		scale := func(namespace, ig, i string) {
			cfg, err := kubectl.GetConfigMap(namespace, "ops-scale")
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Metadata.Name).To(Equal("ops-scale"))
			cfg.Data["ops"] = `- type: replace
  path: /instance_groups/name=` + ig + `?/instances
  value: ` + i

			err = kubectl.ApplyYAML(namespace, "configmap", &cfg)
			Expect(err).ToNot(HaveOccurred())
		}

		checkEntanglement := func(podName, cmd, expect string) error {
			return kubectl.RunCommandWithCheckString(
				namespace, podName,
				cmd,
				expect,
			)
		}

		getPodName := func(namespace, selector string) string {
			podNames, err := kubectl.GetPodNames(namespace, selector)
			Expect(err).ToNot(HaveOccurred())
			//Expect(podNames).To(HaveLen(1))
			Expect(podNames[0]).ToNot(Equal(""))
			return podNames[0]
		}

		upgradeDeploymentToCurrent := func() {
			dir, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			chartPath := fmt.Sprintf("%s%s", dir, "/../../helm/cf-operator")

			teardown, err := e2ehelper.UpgradeChart(chartPath, operatorNamespace,
				"--set", fmt.Sprintf("global.monitoredID=%s", namespace),
				"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", namespace))
			Expect(err).ToNot(HaveOccurred())
			teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)
		}

		deployLatestOperator := func() {
			path, teardown, err := e2ehelper.GetChart("quarks/cf-operator")
			Expect(err).ToNot(HaveOccurred())
			teardowns = append(teardowns, teardown)

			namespace, operatorNamespace, teardown, err = e2ehelper.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			teardowns = append(teardowns, teardown)

			teardown, err = e2ehelper.InstallChart(path+"/cf-operator", operatorNamespace,
				"--set", fmt.Sprintf("global.singleNamespace.name=%s", namespace),
				"--set", fmt.Sprintf("global.monitoredID=%s", namespace),
				"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", namespace),
			)
			Expect(err).ToNot(HaveOccurred())
			teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)
		}

		It("deploys and manage a bosh deployment between upgrades", func() {

			// Get latest release and deploy the operator
			deployLatestOperator()

			// Start gora
			applyNamespace(namespace, "bosh-deployment/quarks-gora-e2e.yaml")
			waitReadyNamespace(namespace, "pod/quarks-gora0-0")
			waitReadyNamespace(namespace, "pod/quarks-gora0-1")
			err := kubectl.WaitForService(namespace, "quarks-gora0-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForService(namespace, "quarks-gora0-1")
			Expect(err).ToNot(HaveOccurred())

			By("Doing sanity checks on the latest release")
			// Try scaling
			scale(namespace, "quarks-gora0", "5")
			waitReadyNamespace(namespace, "pod/quarks-gora0-4")
			err = kubectl.WaitForService(namespace, "quarks-gora0-4")
			Expect(err).ToNot(HaveOccurred())

			scale(namespace, "quarks-gora0", "2")
			err = kubectl.WaitForPodDelete(namespace, "quarks-gora0-2")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.Delete("--namespace", namespace, "bdpl", "quarks-gora0-deployment")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitForPodDelete(namespace, "quarks-gora0-0")
			Expect(err).ToNot(HaveOccurred())

			applyNamespace(namespace, "bosh-deployment/quarks-gora-certs-only.yaml")
			err = kubectl.WaitForSecret(namespace, "var-quarks-gora-ssl")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForSecret(namespace, "var-quarks-gora-ssl-ca")
			Expect(err).ToNot(HaveOccurred())

			// generated explicit variables (certificates)
			applyNamespace(namespace, "bosh-deployment/quarks-gora-certificates.yaml")
			waitReadyNamespace(namespace, "pod/quarks-gora1-0")
			waitReadyNamespace(namespace, "pod/quarks-gora1-1")
			err = kubectl.WaitForService(namespace, "quarks-gora1-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForService(namespace, "quarks-gora1-1")
			Expect(err).ToNot(HaveOccurred())

			exists, err := kubectl.SecretExists(namespace, "link-quarks-gora-quarks-gora")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			err = kubectl.Delete("--namespace", namespace, "bdpl", "quarks-gora1-deployment")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitForPodDelete(namespace, "quarks-gora1-0")
			Expect(err).ToNot(HaveOccurred())

			By("Keeping a bosh-deployment composed of errands/sts/entanglement between cfo upgrades")
			applyNamespace(namespace, "bosh-deployment/quarks-gora-errands.yaml")
			waitReadyNamespace(namespace, "pod/quarks-gora-1")

			// Check autoerrand was executed
			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=autoerrand")
			Expect(err).ToNot(HaveOccurred())

			// Trigger manual errand to verify that deployment is behaving correctly
			err = cmdHelper.TriggerQJob(namespace, "smoke")
			Expect(err).ToNot(HaveOccurred())

			By("Running quarks-gora smoke tests with the latest release of cf-operator")
			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=smoke")
			Expect(err).ToNot(HaveOccurred())

			// Check entanglement
			selector := "example=owned-by-bdpl"
			Eventually(func() int {
				podNames, _ := kubectl.GetPodNames(namespace, selector)
				return len(podNames)
			}, 360*time.Second, 10*time.Second).Should(Equal(1))

			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", selector)
			Expect(err).ToNot(HaveOccurred())

			podName := getPodName(namespace, selector)
			waitReady("pod/" + podName)
			Expect(checkEntanglement(podName, "echo $LINK_QUARKS_GORA_PORT", "55556")).ToNot(HaveOccurred())

			By("Upgrading the operator to the current code checkout")
			// Upgrade to the version of the operator from the checkout
			upgradeDeploymentToCurrent()

			// Check entanglement
			scale(namespace, "quarks-gora", "1")

			By("Wait for downscaling")
			err = kubectl.WaitForPodDelete(namespace, "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())

			By("Checking autoerrand was executed")
			// Check autoerrand was executed
			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=autoerrand")
			Expect(err).ToNot(HaveOccurred())

			Eventually(func() int {
				podNames, _ := kubectl.GetPodNames(namespace, selector)
				return len(podNames)
			}, 360*time.Second, 10*time.Second).Should(Equal(1))

			By("Checking entangled deployment is running")
			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", selector)
			Expect(err).ToNot(HaveOccurred())

			// Checking entanglement
			podName = getPodName(namespace, selector)
			waitReady("pod/" + podName)
			Expect(checkEntanglement(podName, "echo $LINK_QUARKS_GORA_PORT", "55556")).ToNot(HaveOccurred())

			// Try scaling
			scale(namespace, "quarks-gora", "3")
			waitReadyNamespace(namespace, "pod/quarks-gora-2")
			err = kubectl.WaitForService(namespace, "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())

			// Check autoerrand was executed
			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=autoerrand")
			Expect(err).ToNot(HaveOccurred())

			By("Running quarks-gora smoke tests from the current cf-operator code")
			// Run smoke tests again (manual-errand)
			err = cmdHelper.TriggerQJob(namespace, "smoke")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=smoke")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
