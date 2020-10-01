package kube_test

import (
	b64 "encoding/base64"
	"fmt"
	"path"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"

	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
)

var _ = Describe("Examples Directory", func() {
	var (
		example      string
		yamlFilePath string
	)

	const pollInterval = 5 * time.Second

	podRestarted := func(podName string, startTime time.Time) {
		err := wait.PollImmediate(pollInterval, kubectl.PollTimeout, func() (bool, error) {
			status, err := kubectl.PodStatus(namespace, podName)
			if err != nil {
				return false, err
			}

			if status == nil || status.StartTime == nil {
				return false, nil
			}

			return status.StartTime.After(startTime), nil
		})

		Expect(err).ToNot(HaveOccurred())
	}

	JustBeforeEach(func() {
		yamlFilePath = example
		if !strings.HasPrefix(example, "/") {
			yamlFilePath = path.Join(examplesDir, example)
		}
		err := cmdHelper.Create(namespace, yamlFilePath)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("quarks-statefulset configs examples", func() {
		BeforeEach(func() {
			example = "quarks-statefulset/qstatefulset_configs.yaml"
		})

		It("creates and updates statefulsets", func() {
			By("Checking for pods")
			waitReady("pod/example-quarks-statefulset-0")
			waitReady("pod/example-quarks-statefulset-1")

			yamlUpdatedFilePath := examplesDir + "quarks-statefulset/qstatefulset_configs_updated.yaml"

			By("Updating the config value used by pods")
			err := cmdHelper.Apply(namespace, yamlUpdatedFilePath)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the updated value in the env")
			err = wait.PollImmediate(pollInterval, kubectl.PollTimeout, func() (bool, error) {
				err := kubectl.RunCommandWithCheckString(namespace, "example-quarks-statefulset-0", "env", "SPECIAL_KEY=value1Updated")
				if err != nil {
					return false, nil
				}
				return true, nil
			})
			Expect(err).ToNot(HaveOccurred(), "polling for example-quarks-statefulset-0 with special key")

			err = kubectl.RunCommandWithCheckString(namespace, "example-quarks-statefulset-1", "env", "SPECIAL_KEY=value1Updated")
			Expect(err).ToNot(HaveOccurred(), "waiting for example-quarks-statefulset-1 with special key")
		})

		It("creates and updates statefulsets even out of a failure situation", func() {
			By("Checking for pods")
			waitReady("pod/example-quarks-statefulset-0")
			waitReady("pod/example-quarks-statefulset-1")

			yamlUpdatedFilePathFailing := examplesDir + "quarks-statefulset/qstatefulset_configs_fail.yaml"
			err := cmdHelper.Apply(namespace, yamlUpdatedFilePathFailing)
			Expect(err).ToNot(HaveOccurred())

			By("Waiting for failed pod")
			err = wait.PollImmediate(pollInterval, kubectl.PollTimeout, func() (bool, error) {
				podStatus, err := kubectl.PodStatus(namespace, "example-quarks-statefulset-1")
				if err != nil {
					return true, err
				}

				return len(podStatus.ContainerStatuses) > 0 &&
					podStatus.ContainerStatuses[0].LastTerminationState.Terminated != nil &&
					podStatus.ContainerStatuses[0].LastTerminationState.Terminated.ExitCode == 1, nil
			})
			Expect(err).ToNot(HaveOccurred(), "polling for example-quarks-statefulset-1")

			yamlUpdatedFilePath := examplesDir + "quarks-statefulset/qstatefulset_configs_updated.yaml"

			By("Updating the config value used by pods")
			err = cmdHelper.Apply(namespace, yamlUpdatedFilePath)
			Expect(err).ToNot(HaveOccurred())

			By("Checking the updated value in the env")
			err = wait.PollImmediate(pollInterval, kubectl.PollTimeout, func() (bool, error) {
				err := kubectl.RunCommandWithCheckString(namespace, "example-quarks-statefulset-0", "env", "SPECIAL_KEY=value1Updated")
				if err != nil {
					return false, nil
				}
				return true, nil
			})
			Expect(err).ToNot(HaveOccurred(), "polling for example-quarks-statefulset-0 with special key")

			err = wait.PollImmediate(pollInterval, kubectl.PollTimeout, func() (bool, error) {
				err := kubectl.RunCommandWithCheckString(namespace, "example-quarks-statefulset-1", "env", "SPECIAL_KEY=value1Updated")
				if err != nil {
					return false, nil
				}
				return true, nil
			})
			Expect(err).ToNot(HaveOccurred(), "polling for example-quarks-statefulset-1 with special key")
		})

		It("it labels the first pod as active", func() {
			yamlUpdatedFilePath := examplesDir + "quarks-statefulset/qstatefulset_active_passive.yaml"
			By("Applying a quarkstatefulset with active-passive probe")
			err := cmdHelper.Apply(namespace, yamlUpdatedFilePath)
			Expect(err).ToNot(HaveOccurred())

			err = kubectl.WaitForPod(namespace, "quarks.cloudfoundry.org/pod-active", "example-quarks-statefulset-0")
			Expect(err).ToNot(HaveOccurred(), "waiting for example-quarks-statefulset-0")
		})

	})

	Context("quarks-statefulset examples", func() {
		BeforeEach(func() {
			example = "quarks-statefulset/qstatefulset_tolerations.yaml"
		})

		It("creates statefulset pods with tolerations defined", func() {
			By("Checking for pods")
			waitReady("pod/example-quarks-statefulset-0")

			tolerations, err := cmdHelper.GetData(namespace, "pod", "example-quarks-statefulset-0", "go-template={{.spec.tolerations}}")
			Expect(err).ToNot(HaveOccurred())
			Expect(tolerations).To(ContainSubstring(string("effect:NoSchedule")))
			Expect(tolerations).To(ContainSubstring(string("key:key")))
			Expect(tolerations).To(ContainSubstring(string("value:value")))

		})
	})

	Context("bosh-deployment service example", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-service.yaml"
		})

		It("creates the deployment and an endpoint", func() {
			By("Checking for pods")
			waitReady("pod/nats-0")
			waitReady("pod/nats-1")

			err := kubectl.WaitForService(namespace, "nats-service")
			Expect(err).ToNot(HaveOccurred())

			ip0, err := cmdHelper.GetData(namespace, "pod", "nats-0", "go-template={{.status.podIP}}")
			Expect(err).ToNot(HaveOccurred())

			ip1, err := cmdHelper.GetData(namespace, "pod", "nats-1", "go-template={{.status.podIP}}")
			Expect(err).ToNot(HaveOccurred())

			out, err := cmdHelper.GetData(namespace, "endpoints", "nats-service", "go-template=\"{{(index .subsets 0).addresses}}\"")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring(string(ip0)))
			Expect(out).To(ContainSubstring(string(ip1)))
		})
	})

	Context("bosh-deployment example", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment.yaml"
		})

		It("deploys two pods", func() {
			waitReady("pod/nats-0")
			waitReady("pod/nats-1")
		})
	})

	Context("bosh-deployment with a custom variable example", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-custom-variable.yaml"
		})

		It("uses the custom variable", func() {
			By("Checking for pods")
			waitReady("pod/nats-0")
			waitReady("pod/nats-1")

			By("Checking the value in the config file")
			// password is stored in line 24
			password, err := cmdHelper.RunCommandWithOutput(namespace, "nats-1", "awk 'NR == 24 {print substr($2,2,17)}' /var/vcap/jobs/nats/config/nats.conf")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.TrimSuffix(password, "\n")).To(Equal("a-custom-password"))
		})

	})

	Context("bosh-deployment with pre-render ops files", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-pre-ops.yaml"
		})

		It("applies the ops file", func() {
			By("Checking for pods")
			waitReady("pod/nats-0")

			By("Checking the value in the config file")

			envSetByOps, err := cmdHelper.RunCommandWithOutput(namespace, "nats-0", "env")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.TrimSuffix(envSetByOps, "\n")).To(ContainSubstring("deadbeef"))
		})

	})

	Context("bosh-deployment with user variables", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-user-variable.yaml"
		})

		It("uses the user's variables instead of generating new ones", func() {
			By("Checking for pods")
			waitReady("pod/nats-0")

			// Check that we didn't create new secrets
			sd, err := cmdHelper.GetData(namespace, "secret", "var-nats-password", "go-template={{.data}}")
			Expect(err).ToNot(HaveOccurred())
			Expect(sd).To(BeEmpty())
			sd, err = cmdHelper.GetData(namespace, "secret", "var-nats-ca", "go-template={{.data}}")
			Expect(err).ToNot(HaveOccurred())
			Expect(sd).To(BeEmpty())
			sd, err = cmdHelper.GetData(namespace, "secret", "var-nats-cert", "go-template={{.data}}")
			Expect(err).ToNot(HaveOccurred())
			Expect(sd).To(BeEmpty())

			// Check that the manifest contains the user's certs and passwords
			outSecret, err := cmdHelper.GetData(namespace, "secret", "desired-manifest-v1", `go-template={{index .data "manifest.yaml"}}`)
			Expect(err).ToNot(HaveOccurred())
			desiredManifest, _ := b64.StdEncoding.DecodeString(string(outSecret))
			Expect(string(desiredManifest)).To(ContainSubstring("password: deadbeef"))
			Expect(string(desiredManifest)).To(ContainSubstring("ca_cert: my-ca-cert-data"))
			Expect(string(desiredManifest)).To(ContainSubstring("ca_key: my-ca-private-key"))
			Expect(string(desiredManifest)).To(ContainSubstring("cert: my-cert-data"))
			Expect(string(desiredManifest)).To(ContainSubstring("key: my-private-key-data"))
			Expect(string(desiredManifest)).To(ContainSubstring("cert_ca: my-ca-cert-data"))
		})
	})

	Context("bosh-deployment with a custom variable and logging sidecar disable example", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-custom-variable-disable-sidecar.yaml"
		})
		It("disables the logging sidecar", func() {
			By("Checking for pods")
			waitReady("pod/nats-0")
			waitReady("pod/nats-1")

			By("Ensure only one container exists")
			containerName, err := cmdHelper.GetData(namespace, "pod", "nats-0", "jsonpath={range .spec.containers[*]}{.name}")
			Expect(err).ToNot(HaveOccurred())
			Expect(containerName).To(ContainSubstring("nats-nats"))
			Expect(containerName).ToNot(ContainSubstring("logs"))

			containerName, err = cmdHelper.GetData(namespace, "pod", "nats-1", "jsonpath={range .spec.containers[*]}{.name}")
			Expect(err).ToNot(HaveOccurred())
			Expect(containerName).To(ContainSubstring("nats-nats"))
			Expect(containerName).ToNot(ContainSubstring("logs"))
		})
	})

	Context("bosh-deployment with an implicit variable example", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-implicit-variable.yaml"
		})

		It("updates deployment when implicit variable changes", func() {
			By("Checking for pods")
			waitReady("pod/nats-0")
			status, err := kubectl.PodStatus(namespace, "nats-0")
			Expect(err).ToNot(HaveOccurred(), "error getting pod status")
			startTime := status.StartTime

			By("Updating implicit variable")
			implicitVariablePath := examplesDir + "bosh-deployment/implicit-variable-updated.yaml"
			err = cmdHelper.Apply(namespace, implicitVariablePath)
			Expect(err).ToNot(HaveOccurred())

			By("Checking for pod restart")
			podRestarted("nats-0", startTime.Time)
		})
	})

	Context("bosh-deployment with an implicit variable used by an explicit variable example", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-implicit-in-explicit-variable.yaml"
		})

		It("updates quarks secret when implicit variable changes, then deployment updates", func() {
			By("Checking for pods")
			waitReady("pod/nats-0")
			status, err := kubectl.PodStatus(namespace, "nats-0")
			Expect(err).ToNot(HaveOccurred(), "error getting pod status")
			startTime := status.StartTime

			By("Updating implicit variable")
			implicitVariablePath := examplesDir + "bosh-deployment/implicit-variable-updated.yaml"
			err = cmdHelper.Apply(namespace, implicitVariablePath)
			Expect(err).ToNot(HaveOccurred())

			By("Checking for pod restart")
			podRestarted("nats-0", startTime.Time)
		})
	})

	Context("bosh dns example", func() {
		BeforeEach(func() {
			example = "bosh-deployment/boshdeployment-with-bosh-dns.yaml"
		})

		It("resolves BOSH DNS placeholder aliases", func() {
			By("Getting expected IP")
			podName := "nats-0"
			waitReady(fmt.Sprintf("pod/%s", podName))
			serviceName := "nats-0"
			service, err := kubectl.Service(namespace, serviceName)
			Expect(err).ToNot(HaveOccurred())

			By("DNS lookup")
			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", "app=bosh-dns")
			Expect(err).ToNot(HaveOccurred())
			placeholderNames := []string{
				"nats-0.myplaceholderalias.service.cf.internal.",
				"nats-0.myplaceholderalias.service.cf.internal",
			}

			for _, name := range placeholderNames {
				err = kubectl.RunCommandWithCheckString(namespace, podName, fmt.Sprintf("nslookup %s", name), service.Spec.ClusterIP)
				Expect(err).ToNot(HaveOccurred())
			}

			By("negativ DNS lookup")
			unresolvableNames := []string{
				"myplaceholderalias.",
				"myplaceholderalias.service.",
			}

			for _, name := range unresolvableNames {
				err = kubectl.RunCommandWithCheckString(namespace, podName, fmt.Sprintf("nslookup %s || true", name), "NXDOMAIN")
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("resolves BOSH DNS aliases", func() {
			By("Getting expected IP")
			podName := "nats-0"
			waitReady(fmt.Sprintf("pod/%s", podName))
			podStatus, err := kubectl.PodStatus(namespace, podName)
			Expect(err).ToNot(HaveOccurred())

			By("DNS lookup")
			err = kubectl.WaitLabelFilter(namespace, "ready", "pod", "app=bosh-dns")
			Expect(err).ToNot(HaveOccurred())
			cnames := []string{
				"nats.service.cf.internal.",
				"nats.service.cf.internal",
			}
			for _, name := range cnames {
				err = kubectl.RunCommandWithCheckString(namespace, podName, fmt.Sprintf("nslookup %s", name), podStatus.PodIP)
				Expect(err).ToNot(HaveOccurred())
			}

			By("negative DNS lookup")
			unresolvableWildCardNames := []string{
				"foo.nats.service.cf.internal.",
				"foo.nats.service.cf.internal",
			}
			for _, name := range unresolvableWildCardNames {
				err = kubectl.RunCommandWithCheckString(namespace, podName, fmt.Sprintf("nslookup %s || true", name), "no servers could be reached")
				Expect(err).NotTo(HaveOccurred())
			}

			unresolvableCNAMES := []string{
				"myplaceholderalias.",
				"myplaceholderalias.service.",
			}
			for _, name := range unresolvableCNAMES {
				err = kubectl.RunCommandWithCheckString(namespace, podName, fmt.Sprintf("nslookup %s || true", name), "NXDOMAIN")
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})

})
