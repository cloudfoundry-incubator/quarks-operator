package upgrade_suite_test

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
	var monitoredID string

	JustBeforeEach(func() {
		monitoredID = ""

		// Delete previous deployment and reset teardowns
		err := e2ehelper.TearDownAll(teardowns)
		if err != nil {
			fmt.Printf("Failures while cleaning up test environment:\n %v", err)
		}

		teardowns = []e2ehelper.TearDownFunc{}
		// Delete also CRDs
		_ = kubectl.Delete("crds", "boshdeployments.quarks.cloudfoundry.org")
		_ = kubectl.Delete("crds", "quarksjobs.quarks.cloudfoundry.org")
		_ = kubectl.Delete("crds", "quarkssecrets.quarks.cloudfoundry.org")
		_ = kubectl.Delete("crds", "quarksstatefulsets.quarks.cloudfoundry.org")
		teardown, err := e2ehelper.AddHelmRepo("quarks", "https://cloudfoundry-incubator.github.io/quarks-helm")
		Expect(err).ToNot(HaveOccurred())
		teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)
	})

	Context("latest available chart", func() {
		scale := func(namespace, i string) {
			cfg, err := kubectl.GetConfigMap(namespace, "ops-scale")
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg.Metadata.Name).To(Equal("ops-scale"))
			cfg.Data["ops"] = `- type: replace
  path: /instance_groups/name=quarks-gora?/instances
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
			Expect(podNames[0]).ToNot(Equal(""))
			return podNames[0]
		}

		upgradeDeploymentToCurrent := func(singlenamespace bool) {
			dir, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			chartPath := fmt.Sprintf("%s%s", dir, "/../../../helm/cf-operator")
			if singlenamespace {
				_, err := e2ehelper.UpgradeChart(chartPath, operatorNamespace,
					"--set", fmt.Sprintf("global.singleNamespace.name=%s", namespace),
					"--set", fmt.Sprintf("global.monitoredID=%s", monitoredID),
					"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", monitoredID))
				Expect(err).ToNot(HaveOccurred())
			} else {
				_, err := e2ehelper.UpgradeChart(chartPath, operatorNamespace,
					"--set", fmt.Sprintf("global.singleNamespace.create=%s", "false"),
					"--set", fmt.Sprintf("global.monitoredID=%s", monitoredID),
					"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", monitoredID))
				Expect(err).ToNot(HaveOccurred())
			}
		}

		deployLatestOperator := func(singlenamespace bool) {
			path, teardown, err := e2ehelper.GetChart("quarks/cf-operator")
			Expect(err).ToNot(HaveOccurred())
			teardowns = append(teardowns, teardown)

			monitoredID, operatorNamespace, teardown, err = e2ehelper.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			teardowns = append(teardowns, teardown)

			if singlenamespace {
				teardown, err = e2ehelper.InstallChart(path+"/cf-operator", operatorNamespace,
					"--set", fmt.Sprintf("global.singleNamespace.name=%s", monitoredID),
					"--set", fmt.Sprintf("global.monitoredID=%s", monitoredID),
					"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", monitoredID),
				)
			} else {
				teardown, err = e2ehelper.InstallChart(path+"/cf-operator", operatorNamespace,
					"--set", fmt.Sprintf("global.singleNamespace.create=%s", "false"),
					"--set", fmt.Sprintf("global.monitoredID=%s", monitoredID),
					"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", monitoredID),
				)
			}

			Expect(err).ToNot(HaveOccurred())
			teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)

			if !singlenamespace {
				var nsTeardowns []e2ehelper.TearDownFunc

				namespace, nsTeardowns, err = e2ehelper.CreateMonitoredNamespaceFromExistingRole(monitoredID)
				teardowns = append(teardowns, nsTeardowns...)
				Expect(err).ToNot(HaveOccurred())
			} else {
				namespace = monitoredID
			}
		}

		upgradeTest := func(singlenamesapce bool) {
			By("Deploying the latest release of cfo")
			var err error
			// Get latest release and deploy the operator
			deployLatestOperator(singlenamesapce)

			applyNamespace(namespace, "bosh-deployment/quarks-gora-certs.yaml")
			err = kubectl.WaitForSecret(namespace, "var-quarks-gora-ssl")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForSecret(namespace, "var-quarks-gora-ssl-ca")
			Expect(err).ToNot(HaveOccurred())

			//	Start gora
			applyNamespace(namespace, "bosh-deployment/quarks-gora-errands.yaml")
			waitReadyNamespace(namespace, "pod/quarks-gora-0")
			waitReadyNamespace(namespace, "pod/quarks-gora-1")
			err = kubectl.WaitForService(namespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.WaitForService(namespace, "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())

			By("Doing sanity checks on the latest release")
			exists, err := kubectl.SecretExists(namespace, "link-quarks-gora-quarks-gora")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			// Try scaling
			scale(namespace, "5")
			waitReadyNamespace(namespace, "pod/quarks-gora-4")
			err = kubectl.WaitForService(namespace, "quarks-gora-4")
			Expect(err).ToNot(HaveOccurred())

			scale(namespace, "2")
			err = kubectl.WaitForPodDelete(namespace, "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.Delete("--namespace", namespace, "bdpl", "gora-test-deployment")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitForPodDelete(namespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())

			By("Keeping a bosh-deployment composed of errands/sts/entanglement between cfo upgrades")
			applyNamespace(namespace, "bosh-deployment/quarks-gora-errands.yaml")
			waitReadyNamespace(namespace, "pod/quarks-gora-1")
			Eventually(func() bool {
				exists, _ := kubectl.Exists(namespace, "qjob", "smoke")
				return exists
			}, 360*time.Second, 10*time.Second).Should(BeTrue())

			exists, err = kubectl.ServiceExists(namespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			// Check autoerrand was executed
			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=autoerrand")
			Expect(err).ToNot(HaveOccurred())

			By("Running quarks-gora smoke tests with the latest release of cf-operator")
			// Trigger manual errand to verify that deployment is behaving correctly
			err = cmdHelper.TriggerQJob(namespace, "smoke")
			Expect(err).ToNot(HaveOccurred())

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

			By("Checking quarks-gora is up")
			exists, err = kubectl.PodExists(namespace, "quarks.cloudfoundry.org/deployment-name=gora-test-deployment", "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			By("Upgrading the operator to the current code checkout")
			// Upgrade to the version of the operator from the checkout
			upgradeDeploymentToCurrent(singlenamesapce)

			waitReadyNamespace(namespace, "pod/quarks-gora-1")

			By("Checking quarks-gora is up after upgrade")
			exists, err = kubectl.PodExists(namespace, "quarks.cloudfoundry.org/deployment-name=gora-test-deployment", "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			By("Scaling quarks-gora after the upgrade")
			scale(namespace, "1")

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
			scale(namespace, "3")
			waitReadyNamespace(namespace, "pod/quarks-gora-2")
			err = kubectl.WaitForService(namespace, "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())

			By("Checking if secrets are still present")
			for _, s := range []string{"link-quarks-gora-server-data",
				"var-custom-password", "var-gora-ca",
				"link-quarks-gora-server-data",
				"var-gora-cert",
				"var-gora-password",
				"var-quarks-gora-ssl",
				"test-gora",
			} {
				exists, err = kubectl.SecretExists(namespace, s)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(BeTrue())
			}

			// Check autoerrand was executed
			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=autoerrand")
			Expect(err).ToNot(HaveOccurred())

			By("Running quarks-gora smoke tests from the current cf-operator code")
			// Run smoke tests again (manual-errand) after the cf-operator upgrade to verify that certificates and variables interpolation are working
			// as expected, and our deployment is still accessible
			err = cmdHelper.TriggerQJob(namespace, "smoke")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=smoke")
			Expect(err).ToNot(HaveOccurred())
		}

		It("deploys and manage a bosh deployment between upgrades with multiple namespaces", func() {
			upgradeTest(false)
		})

		It("deploys and manage a bosh deployment between upgrades with singleNamespace", func() {
			upgradeTest(true)
		})

	})
})
