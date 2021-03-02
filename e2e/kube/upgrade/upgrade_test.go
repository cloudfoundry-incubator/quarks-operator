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

// This cannot run in parallel
var _ = Describe("Quarks Upgrade test", func() {
	var (
		monitoredID       string
		namespace         string
		operatorNamespace string
	)

	// Delete CRDs to make sure the cluster is pristine
	BeforeEach(func() {
		_ = kubectl.Delete("crds", "boshdeployments.quarks.cloudfoundry.org")
		_ = kubectl.Delete("crds", "quarksjobs.quarks.cloudfoundry.org")
		_ = kubectl.Delete("crds", "quarkssecrets.quarks.cloudfoundry.org")
		_ = kubectl.Delete("crds", "quarksstatefulsets.quarks.cloudfoundry.org")
	})

	When("upgrading quarks from the latest helm release to the current checkout", func() {
		selector := "example=owned-by-bdpl"

		// upgrade to the locally build helm release
		upgradeOperatorToCurrent := func() {
			dir, err := os.Getwd()
			Expect(err).ToNot(HaveOccurred())

			chartPath := fmt.Sprintf("%s%s", dir, "/../../../helm/quarks")
			_, err = e2ehelper.UpgradeChart(chartPath, operatorNamespace,
				"--set", fmt.Sprintf("global.singleNamespace.name=%s", namespace),
				"--set", fmt.Sprintf("global.monitoredID=%s", monitoredID),
				"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", monitoredID),
				"--set", fmt.Sprintf("corednsServiceAccount.name=%s-%s", namespace, "coredns-quarks"),
			)
			Expect(err).ToNot(HaveOccurred())
		}

		// installLatestOperator fetches latest helm release and deploys the operator
		installLatestOperator := func() string {
			path, teardown, err := e2ehelper.GetChart("quarks/quarks")
			Expect(err).ToNot(HaveOccurred())
			teardowns = append(teardowns, teardown)

			monitoredID, operatorNamespace, teardown, err = e2ehelper.CreateNamespace()
			Expect(err).ToNot(HaveOccurred())
			teardowns = append(teardowns, teardown)

			// uses the default 'singleNamespace' setup from our helm templates
			teardown, err = e2ehelper.InstallChart(path+"/quarks", operatorNamespace,
				"--set", fmt.Sprintf("global.singleNamespace.name=%s", monitoredID),
				"--set", fmt.Sprintf("global.monitoredID=%s", monitoredID),
				"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", monitoredID),
				"--set", fmt.Sprintf("corednsServiceAccount.name=%s-%s", monitoredID, "coredns-quarks"),
			)
			Expect(err).ToNot(HaveOccurred())
			teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)
			return monitoredID
		}

		checkServices := func() {
			services := []string{"quarks-gora", "quarks-gora-0", "quarks-gora-1", "quarks-gora-2"}
			for _, svc := range services {
				err := kubectl.WaitForService(namespace, svc)
				Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to wait for service '%s' creation", svc))
			}

			Eventually(func() error {
				for _, svc := range services {
					works, err := kubectl.ServiceWorks(namespace, svc)
					if err != nil {
						return err
					}
					if !works {
						return fmt.Errorf("ServiceWorks returned false for service '%s'", svc)
					}
				}
				return nil
			}).Should(BeNil(), "DNS lookup on services failed")
		}

		// exerciseGora simulates a real-world deployment checking
		// deployment of pods, working of service and delete of pods.
		exerciseGora := func() {
			apply(namespace, "bosh-deployment/quarks-gora-errands.yaml")
			waitReady(namespace, "pod/quarks-gora-2")

			By("Checking services")
			checkServices()

			By("Doing sanity checks on the latest release")
			exists, err := kubectl.SecretExists(namespace, "link-quarks-gora-quarks-gora")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			// Try scaling
			scale(namespace, "5")
			waitReady(namespace, "pod/quarks-gora-4")
			err = kubectl.WaitForService(namespace, "quarks-gora-4")
			Expect(err).ToNot(HaveOccurred())

			scale(namespace, "2")
			err = kubectl.WaitForPodDelete(namespace, "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.Delete("--namespace", namespace, "bdpl", "gora-test-deployment")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitForPodDelete(namespace, "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitForPodDelete(namespace, "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())
		}

		// deployGora deploys the gora release
		deployGora := func() {
			apply(namespace, "bosh-deployment/quarks-gora-errands.yaml")
			waitReady(namespace, "pod/quarks-gora-2")

			Eventually(func() bool {
				exists, _ := kubectl.Exists(namespace, "qjob", "smoke")
				return exists
			}, 360*time.Second, 10*time.Second).Should(BeTrue())

			By("Checking services")
			checkServices()

			// Check autoerrand was executed
			err := kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=autoerrand")
			Expect(err).ToNot(HaveOccurred())

			By("Running quarks-gora smoke tests with the latest release of quarks")
			// Trigger manual errand to verify that deployment is behaving correctly
			err = cmdHelper.TriggerQJob(namespace, "smoke")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=smoke")
			Expect(err).ToNot(HaveOccurred())

			// Check entanglement
			Eventually(func() int {
				podNames, _ := kubectl.GetPodNames(namespace, selector)
				return len(podNames)
			}, 360*time.Second, 10*time.Second).Should(Equal(1))

			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", selector)
			Expect(err).ToNot(HaveOccurred())

			podName := getPodName(namespace, selector)
			waitReady(namespace, "pod/"+podName)
			Expect(checkEntanglement(namespace, podName, "echo $LINK_QUARKS_GORA_PORT", "55556")).ToNot(HaveOccurred())

			By("Checking quarks-gora is up")
			exists, err := kubectl.PodExists(namespace, "quarks.cloudfoundry.org/deployment-name=gora-test-deployment", "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		}

		BeforeEach(func() {
			By("Deploying the latest release of cf-operator")
			namespace = installLatestOperator()

			By("setting up certs", func() {
				apply(namespace, "bosh-deployment/quarks-gora-certs.yaml")
				err := kubectl.WaitForSecret(namespace, "var-quarks-gora-ssl")
				Expect(err).ToNot(HaveOccurred())
				err = kubectl.WaitForSecret(namespace, "var-quarks-gora-ssl-ca")
				Expect(err).ToNot(HaveOccurred())
			})

			By("Excersing and deploying Gora")
			exerciseGora()
			deployGora()
		})

		It("manages the bosh deployment HA between upgrades", func() {
			upgradeOperatorToCurrent()

			By("Checking quarks-gora is up after upgrade")
			exists, err := kubectl.PodExists(namespace, "quarks.cloudfoundry.org/deployment-name=gora-test-deployment", "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())

			By("Checking services")
			checkServices()

			By("Restart pods")
			err = kubectl.Delete("--namespace", namespace, "pod", "quarks-gora-0")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.Delete("--namespace", namespace, "pod", "quarks-gora-1")
			Expect(err).ToNot(HaveOccurred())
			err = kubectl.Delete("--namespace", namespace, "pod", "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())

			By("Checking services again")
			checkServices()

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
			podName := getPodName(namespace, selector)
			waitReady(namespace, "pod/"+podName)
			Expect(checkEntanglement(namespace, podName, "echo $LINK_QUARKS_GORA_PORT", "55556")).ToNot(HaveOccurred())

			// Try scaling
			scale(namespace, "3")
			waitReady(namespace, "pod/quarks-gora-2")
			err = kubectl.WaitForService(namespace, "quarks-gora-2")
			Expect(err).ToNot(HaveOccurred())

			By("Checking if secrets are still present")
			for _, s := range []string{"link-quarks-gora-server-data",
				"link-quarks-gora-server-data",
				"var-example-cert",
				"var-gora-password",
				"var-quarks-gora-ssl",
				"var-quarks-gora-ssl-ca",
				"var-user-provided-password",
			} {
				exists, err = kubectl.SecretExists(namespace, s)
				Expect(err).ToNot(HaveOccurred(), "error fetching secret '%s'", s)
				Expect(exists).To(BeTrue(), "secret '%s' doesn't exist", s)
			}

			// Check autoerrand was executed
			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=autoerrand")
			Expect(err).ToNot(HaveOccurred())

			By("Running quarks-gora smoke tests from the current quarks code")
			// Run smoke tests again (manual-errand) after the quarks upgrade to verify that certificates and variables interpolation are working
			// as expected, and our deployment is still accessible
			err = cmdHelper.TriggerQJob(namespace, "smoke")
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitLabelFilter(namespace, "complete", "pod", "quarks.cloudfoundry.org/qjob-name=smoke")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
