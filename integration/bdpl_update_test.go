package integration_test

import (
	"fmt"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	bm "code.cloudfoundry.org/quarks-operator/testing/boshmanifest"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

var _ = Describe("BDPL updates", func() {
	const (
		deploymentName = "test"
		manifestName   = "manifest"
	)

	var tearDowns []machine.TearDownFunc

	AfterEach(func() {
		Expect(env.TearDownAll(tearDowns)).To(Succeed())
	})

	Context("when updating a deployment", func() {

		opOneInstance := `- type: replace
  path: /instance_groups/name=nats?/instances
  value: 1
`
		BeforeEach(func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMap(manifestName, bm.NatsSmall))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment(deploymentName, manifestName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, deploymentName, "nats", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
		})

		Context("by adding an ops file to the bdpl custom resource", func() {
			BeforeEach(func() {
				tearDown, err := env.CreateConfigMap(env.Namespace, env.CustomOpsConfigMap("test-ops", opOneInstance))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				bdpl, err := env.GetBOSHDeployment(env.Namespace, deploymentName)
				Expect(err).NotTo(HaveOccurred())
				bdpl.Spec.Ops = []bdv1.ResourceReference{{Name: "test-ops", Type: bdv1.ConfigMapReference}}

				_, _, err = env.UpdateBOSHDeployment(env.Namespace, *bdpl)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the deployment", func() {
				// nats is a special release which consumes itself. So, whenever these is change related instances or azs
				// nats statefulset gets updated 2 times. TODO: need to fix this later.
				err := env.WaitForInstanceGroupVersions(env.Namespace, deploymentName, "nats", 1, "2", "3")
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
			})
		})

		Context("by adding an additional explicit variable", func() {
			BeforeEach(func() {
				cm, err := env.GetConfigMap(env.Namespace, manifestName)
				Expect(err).NotTo(HaveOccurred())
				cm.Data["manifest"] = bm.NatsExplicitVar
				_, _, err = env.UpdateConfigMap(env.Namespace, *cm)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create a new secret for the variable", func() {
				err := env.WaitForSecret(env.Namespace, "var-nats-password")
				Expect(err).NotTo(HaveOccurred(), "error waiting for new generated variable secret")
			})
		})

		Context("by adding a new env var to BPM via quarks job properties", func() {
			opReplaceEnv := `- type: replace
  path: /instance_groups/name=nats/jobs/name=nats/properties/quarks?
  value:
    envs:
    - name: XPOD_IPX
      valueFrom:
        fieldRef:
          apiVersion: v1
          fieldPath: status.podIP
`
			BeforeEach(func() {
				tearDown, err := env.CreateSecret(env.Namespace, env.CustomOpsSecret("ops-bpm", opReplaceEnv))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				bdpl, err := env.GetBOSHDeployment(env.Namespace, deploymentName)
				Expect(err).NotTo(HaveOccurred())
				bdpl.Spec.Ops = []bdv1.ResourceReference{{Name: "ops-bpm", Type: bdv1.SecretReference}}
				Eventually(func() error {
					_, _, err = env.UpdateBOSHDeployment(env.Namespace, *bdpl)
					return err
				}).Should(BeNil())
			})

			It("should add the env var to the container", func() {
				err := env.WaitForInstanceGroupVersions(env.Namespace, deploymentName, "nats", 2, "2", "3")
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

				Eventually(func() []string {
					pod, err := env.GetPod(env.Namespace, "nats-1")
					Expect(err).NotTo(HaveOccurred())
					return env.EnvKeys(pod.Spec.Containers)
				}).Should(ContainElement("XPOD_IPX"))
			})
		})

		Context("by adding a new port via quarks job properties", func() {
			opReplacePorts := `- type: replace
  path: /instance_groups/name=nats/jobs/name=nats/properties/quarks/ports?/-
  value:
    name: "fake-port"
    protocol: "TCP"
    internal: 6443
`
			BeforeEach(func() {
				tearDown, err := env.CreateSecret(env.Namespace, env.CustomOpsSecret("ops-ports", opReplacePorts))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				bdpl, err := env.GetBOSHDeployment(env.Namespace, deploymentName)
				Expect(err).NotTo(HaveOccurred())
				bdpl.Spec.Ops = []bdv1.ResourceReference{{Name: "ops-ports", Type: bdv1.SecretReference}}
				_, _, err = env.UpdateBOSHDeployment(env.Namespace, *bdpl)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the service with new port", func() {
				err := env.WaitForSecret(env.Namespace, "bpm.nats-v2")
				Expect(err).NotTo(HaveOccurred(), "error waiting for new bpm config")

				err = env.WaitForServiceVersion(env.Namespace, "nats", "2")
				Expect(err).NotTo(HaveOccurred(), "error waiting for service from deployment")

				svc, err := env.GetService(env.Namespace, "nats")
				Expect(err).NotTo(HaveOccurred())
				Expect(svc.Spec.Ports).To(ContainElement(corev1.ServicePort{
					Name:       "fake-port",
					Port:       6443,
					Protocol:   "TCP",
					TargetPort: intstr.FromInt(6443),
				}))
			})
		})

		Context("deployment downtime", func() {
			// This test uses a NodePort to work around private network issues.
			// This test cannot be executed from remote.
			// the k8s node must be reachable from the
			// machine running the test.

			// We define "downtime" as more than 5 errors, while checking every 200ms during the update
			It("should be zero", func() {
				By("Setting up a NodePort service")
				svc, err := env.GetService(env.Namespace, "nats-0")
				Expect(err).NotTo(HaveOccurred(), "error retrieving clusterIP service")

				tearDown, err := env.CreateService(env.Namespace, env.NodePortService("nats-service", "nats", svc.Spec.Ports[0].Port))
				defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
				Expect(err).NotTo(HaveOccurred(), "error creating service")

				service, err := env.GetService(env.Namespace, "nats-service")
				Expect(err).NotTo(HaveOccurred(), "error retrieving service")

				nodeIP, err := env.NodeIP()
				Expect(err).NotTo(HaveOccurred(), "error retrieving node ip")

				address := fmt.Sprintf("%s:%d", nodeIP, service.Spec.Ports[0].NodePort)
				err = env.WaitForPortReachable("tcp", address)
				Expect(err).NotTo(HaveOccurred(), "port not reachable")

				By("Setting up a service watcher")
				resultChan := make(chan machine.ChanResult)
				stopChan := make(chan struct{})

				watchNats := func(outChan chan machine.ChanResult, stopChan chan struct{}, uri string) {
					var (
						checkDelay time.Duration = 200 * time.Millisecond // Time between checks in ms
						maxErrors                = 5
						tcpTimeout               = 10 * time.Second // usually around 3min
					)

					i := 0
					errCount := 0
					tick := time.Tick(checkDelay)

					for {
						i++
						select {
						case <-tick:

							_, err := net.DialTimeout("tcp", uri, tcpTimeout)
							if err != nil {
								errCount++
								if errCount >= maxErrors {
									outChan <- machine.ChanResult{
										Error: errors.Wrapf(err, "on try %d, after %d errors", i, errCount),
									}
									return
								}
							}
							time.Sleep(checkDelay)
						case <-stopChan:
							outChan <- machine.ChanResult{
								Error: nil,
							}
							return
						}
					}
				}

				go watchNats(resultChan, stopChan, address)

				By("Triggering an upgrade")
				cm, err := env.GetConfigMap(env.Namespace, manifestName)
				Expect(err).NotTo(HaveOccurred())
				cm.Data["manifest"] = strings.Replace(cm.Data["manifest"], "changeme", "dont", -1)
				_, _, err = env.UpdateConfigMap(env.Namespace, *cm)
				Expect(err).NotTo(HaveOccurred())

				err = env.WaitForInstanceGroup(env.Namespace, deploymentName, "nats", "2", 2)
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

				// Stop the watcher if it's still running
				close(stopChan)

				// Collect result
				result := <-resultChan
				Expect(result.Error).NotTo(HaveOccurred())
			})
		})
	})

	Context("when updating a deployment which uses ops files", func() {
		opOneInstance := `- type: replace
  path: /instance_groups/name=quarks-gora?/instances
  value: 1
`
		BeforeEach(func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMap(manifestName, bm.Gora))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			tearDown, err = env.CreateConfigMap(env.Namespace, env.CustomOpsConfigMap("bosh-ops", opOneInstance))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeploymentWithOps(deploymentName, manifestName, "bosh-ops"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, deploymentName, "quarks-gora", "1", 1)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
		})

		scaleDeployment := func(n string) {
			ops, err := env.GetConfigMap(env.Namespace, "bosh-ops")
			Expect(err).NotTo(HaveOccurred())
			ops.Data["ops"] = `- type: replace
  path: /instance_groups/name=quarks-gora?/instances
  value: ` + n
			_, _, err = env.UpdateConfigMap(env.Namespace, *ops)
			Expect(err).NotTo(HaveOccurred())
		}

		Context("by modifying a referenced ops files", func() {
			It("should update the deployment and respect the instance count", func() {
				scaleDeployment("2")

				By("checking for instance group updated pods")
				err := env.WaitForInstanceGroupVersions(env.Namespace, deploymentName, "quarks-gora", 2, "2", "3")
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

				pods, _ := env.GetInstanceGroupPods(env.Namespace, deploymentName, "quarks-gora")
				Expect(len(pods.Items)).To(Equal(2))

				By("updating the deployment again")
				scaleDeployment("3")

				By("checking if the deployment was again updated")
				Eventually(func() int {
					pods, err := env.GetInstanceGroupPods(env.Namespace, deploymentName, "quarks-gora")
					if err != nil {
						return 0
					}
					return len(pods.Items)
				}).Should(Equal(3))

				pods, _ = env.GetInstanceGroupPods(env.Namespace, deploymentName, "quarks-gora")
				Expect(len(pods.Items)).To(Equal(3))
			})
		})

		Context("by modifying a referenced ops files multiple times", func() {
			It("should update the deployment and respect the instance count", func() {
				scaleDeployment("2")
				scaleDeployment("3")
				scaleDeployment("4")

				By("checking for instance group updated pods")
				err := env.WaitForInstanceGroupVersions(env.Namespace, deploymentName, "quarks-gora", 4, "2")
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
			})
		})
	})

	Context("when updating a deployment with explicit vars", func() {
		opReplacePorts := `- type: replace
  path: /instance_groups/name=nats/jobs/name=nats/properties/quarks/ports?/-
  value:
    name: "fake-port"
    protocol: "TCP"
    internal: 6443
`

		BeforeEach(func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMap(manifestName, bm.NatsExplicitVar))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment(deploymentName, manifestName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, deploymentName, "nats", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
		})

		Context("unnecessary secret updates should not happen", func() {
			It("update the instance group", func() {
				secret, err := env.GetSecret(env.Namespace, "var-nats-password")
				Expect(err).NotTo(HaveOccurred(), "error getting var-nats-password secret")
				passwordv1 := string(secret.Data["password"])

				By("updating the deployment")
				tearDown, err := env.CreateSecret(env.Namespace, env.CustomOpsSecret("ops-ports", opReplacePorts))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				bdpl, err := env.GetBOSHDeployment(env.Namespace, deploymentName)
				Expect(err).NotTo(HaveOccurred())
				bdpl.Spec.Ops = []bdv1.ResourceReference{{Name: "ops-ports", Type: bdv1.SecretReference}}
				_, _, err = env.UpdateBOSHDeployment(env.Namespace, *bdpl)
				Expect(err).NotTo(HaveOccurred())

				By("checking for instance group pods")
				err = env.WaitForInstanceGroup(env.Namespace, deploymentName, "nats", "2", 2)
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

				secret, err = env.GetSecret(env.Namespace, "var-nats-password")
				Expect(err).NotTo(HaveOccurred(), "error getting var-nats-password secret")
				passwordv2 := string(secret.Data["password"])
				Expect(passwordv1).To(Equal(passwordv2))
			})
		})

		Context("by setting a user-defined explicit variable", func() {
			BeforeEach(func() {
				tearDown, err := env.CreateSecret(env.Namespace, env.UserExplicitPassword("my-var", "supersecret"))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				bdpl, err := env.GetBOSHDeployment(env.Namespace, deploymentName)
				Expect(err).NotTo(HaveOccurred())
				bdpl.Spec.Vars = []bdv1.VarReference{{Name: "nats_password", Secret: "my-var"}}
				_, _, err = env.UpdateBOSHDeployment(env.Namespace, *bdpl)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use the value from the user's secret", func() {
				err := env.WaitForSecret(env.Namespace, "desired-manifest-v2")
				Expect(err).NotTo(HaveOccurred(), "error waiting for new desired manifest")

				secret, err := env.GetSecret(env.Namespace, "desired-manifest-v2")
				Expect(err).NotTo(HaveOccurred(), "error getting new desired manifest")

				manifest := string(secret.Data["manifest.yaml"])

				Expect(manifest).To(ContainSubstring("nats_password: supersecret"))
			})

			It("should update when the user's secret changes", func() {
				err := env.WaitForSecret(env.Namespace, "desired-manifest-v2")
				Expect(err).NotTo(HaveOccurred(), "error waiting for new desired manifest")

				_, tearDown, err := env.UpdateSecret(env.Namespace, env.UserExplicitPassword("my-var", "anothersupersecret"))
				Expect(err).NotTo(HaveOccurred(), "error updating user var")
				tearDowns = append(tearDowns, tearDown)

				err = env.WaitForSecret(env.Namespace, "desired-manifest-v3")
				Expect(err).NotTo(HaveOccurred(), "error waiting for new desired manifest")

				secret, err := env.GetSecret(env.Namespace, "desired-manifest-v3")
				Expect(err).NotTo(HaveOccurred(), "error getting new desired manifest")

				manifest := string(secret.Data["manifest.yaml"])

				Expect(manifest).To(ContainSubstring("nats_password: anothersupersecret"))
			})
		})
	})

	Context("when updating a deployment with delete ops file", func() {
		opsDelete := `- type: remove
  path: /instance_groups/name=nats
`
		BeforeEach(func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMap(manifestName, bm.NatsSmall))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment(deploymentName, manifestName))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, deploymentName, "nats", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
		})

		Context("by adding an delete instance group ops file to the bdpl", func() {
			BeforeEach(func() {
				tearDown, err := env.CreateConfigMap(env.Namespace, env.CustomOpsConfigMap("test-ops", opsDelete))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				bdpl, err := env.GetBOSHDeployment(env.Namespace, deploymentName)
				Expect(err).NotTo(HaveOccurred())
				bdpl.Spec.Ops = []bdv1.ResourceReference{{Name: "test-ops", Type: bdv1.ConfigMapReference}}

				_, _, err = env.UpdateBOSHDeployment(env.Namespace, *bdpl)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delete the qsts", func() {
				err := env.WaitForQuarksStatefulSetDelete(env.Namespace, "nats")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Context("when updating a deployment with multiple instance groups", func() {
		It("it should only update correctly and have correct secret versions in volume mounts", func() {
			manifestName := "bosh-manifest-two-instance-groups"
			tearDown, err := env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMap("fooconfigmap", bm.BOSHManifestWithTwoInstanceGroups))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment(manifestName, "fooconfigmap"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf machine.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for nats instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, manifestName, "nats", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for route_registrar instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, manifestName, "route_registrar", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("Updating the deployment")
			cm, err := env.GetConfigMap(env.Namespace, "fooconfigmap")
			Expect(err).NotTo(HaveOccurred())
			cm.Data["manifest"] = strings.Replace(cm.Data["manifest"], "changeme", "dont", -1)
			_, _, err = env.UpdateConfigMap(env.Namespace, *cm)
			Expect(err).NotTo(HaveOccurred())

			By("Checking volume mounts with secret versions")
			Eventually(func() error {
				pod, err := env.GetPod(env.Namespace, "nats-1")
				if err != nil {
					return err
				}
				if pod.Spec.Volumes[4].Secret.SecretName != "ig-resolved.nats-v2" {
					return fmt.Errorf("wrong ig resolved secret version")
				}
				Expect(pod.Spec.InitContainers[2].VolumeMounts[2].Name).To(Equal("ig-resolved"))

				pod, err = env.GetPod(env.Namespace, "route-registrar-0")
				if err != nil {
					return err
				}
				if pod.Spec.Volumes[4].Secret.SecretName != "ig-resolved.route-registrar-v2" {
					return fmt.Errorf("wrong ig resolved secret version")
				}
				Expect(pod.Spec.InitContainers[2].VolumeMounts[2].Name).To(Equal("ig-resolved"))
				return nil
			}).Should(Succeed())
		})
	})
})
