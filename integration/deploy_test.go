package integration_test

import (
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	bm "code.cloudfoundry.org/cf-operator/testing/boshmanifest"
)

var _ = Describe("Deploy", func() {
	Context("when using the default configuration", func() {
		stsName := "test-nats-v1"
		headlessSvcName := "test-nats"
		clusterIpSvcName := "test-nats-0"

		It("should deploy a pod and create services", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test", "manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for services")
			svc, err := env.GetService(env.Namespace, headlessSvcName)
			Expect(err).NotTo(HaveOccurred(), "error getting service for instance group")
			Expect(svc.Spec.Selector).To(Equal(map[string]string{bdm.LabelInstanceGroupName: "nats"}))
			Expect(svc.Spec.Ports).NotTo(BeEmpty())
			Expect(svc.Spec.Ports[0].Name).To(Equal("nats"))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(4222)))

			svc, err = env.GetService(env.Namespace, clusterIpSvcName)
			Expect(err).NotTo(HaveOccurred(), "error getting service for instance group")
			Expect(svc.Spec.Selector).To(Equal(map[string]string{
				bdm.LabelInstanceGroupName: "nats",
				essv1.LabelAZIndex:         "0",
				essv1.LabelPodOrdinal:      "0",
			}))
			Expect(svc.Spec.Ports).NotTo(BeEmpty())
			Expect(svc.Spec.Ports[0].Name).To(Equal("nats"))
			Expect(svc.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(4222)))
		})

		It("should deploy manifest with multiple ops correctly", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			tearDown, err = env.CreateConfigMap(env.Namespace, env.InterpolateOpsConfigMap("bosh-ops"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			tearDown, err = env.CreateSecret(env.Namespace, env.InterpolateOpsSecret("bosh-ops-secret"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.InterpolateBOSHDeployment("test", "manifest", "bosh-ops", "bosh-ops-secret"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "1", 3)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			sts, err := env.GetStatefulSet(env.Namespace, stsName)
			Expect(err).NotTo(HaveOccurred(), "error getting statefulset for deployment")
			Expect(*sts.Spec.Replicas).To(BeEquivalentTo(3))
		})

	})

	Context("when using pre-render scripts", func() {
		podName := "test-nats-v1-0"

		It("it should run them", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "manifest"},
				Data: map[string]string{
					"manifest": bm.NatsSmallWithPatch,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test", "manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for init container")

			err = env.WaitForInitContainerRunning(env.Namespace, podName, "bosh-pre-start-nats")
			Expect(err).NotTo(HaveOccurred(), "error waiting for pre-start init container from pod")

			Expect(env.WaitForPodContainerLogMsg(env.Namespace, podName, "bosh-pre-start-nats", "this file was patched")).To(BeNil(), "error getting logs from drain_watch process")
		})
	})

	Context("when BPM has pre-start hooks configured", func() {
		It("should run pre-start script in an init container", func() {

			By("Checking if minikube is present")
			_, err := exec.Command("/bin/sh", "-c", "kubectl config view | grep minikube").Output()
			if err == nil {
				Skip("Skipping because this test is not supported in minikube")
			}

			tearDown, err := env.CreateConfigMap(env.Namespace, corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "garden-manifest"},
				Data:       map[string]string{"manifest": bm.GardenRunc},
			})
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test-bdpl", "garden-manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test-bdpl", "garden-runc", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for containers")
			pods, _ := env.GetPods(env.Namespace, "fissile.cloudfoundry.org/instance-group-name=garden-runc")
			Expect(len(pods.Items)).To(Equal(2))
			pod := pods.Items[1]
			Expect(pod.Spec.InitContainers).To(HaveLen(5))
			Expect(pod.Spec.InitContainers[4].Args).To(ContainElement("/var/vcap/jobs/garden/bin/bpm-pre-start"))
			Expect(pod.Spec.InitContainers[4].Command[0]).To(Equal("/bin/sh"))
		})
	})

	Context("when BOSH has pre-start hooks configured", func() {
		It("should run pre-start script in an init container", func() {

			tearDown, err := env.CreateConfigMap(env.Namespace, corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "cfrouting-manifest"},
				Data:       map[string]string{"manifest": bm.CFRouting},
			})
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test-bph", "cfrouting-manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test-bph", "route_registrar", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for containers")
			pods, _ := env.GetPods(env.Namespace, "fissile.cloudfoundry.org/instance-group-name=route_registrar")
			Expect(len(pods.Items)).To(Equal(2))

			pod := pods.Items[1]
			Expect(pod.Spec.InitContainers).To(HaveLen(4))
			Expect(pod.Spec.InitContainers[3].Name).To(Equal("bosh-pre-start-route-registrar"))
		})
	})

	Context("when job name contains an underscore", func() {
		var tearDowns []environment.TearDownFunc

		AfterEach(func() {
			Expect(env.TearDownAll(tearDowns)).To(Succeed())
		})

		It("should apply naming guidelines", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "router-manifest"},
				Data:       map[string]string{"manifest": bm.CFRouting},
			})
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test-bdpl", "router-manifest"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)

			By("checking for instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test-bdpl", "route_registrar", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for containers")
			pods, _ := env.GetPods(env.Namespace, "fissile.cloudfoundry.org/instance-group-name=route_registrar")
			Expect(len(pods.Items)).To(Equal(2))
			Expect(pods.Items[0].Spec.Containers).To(HaveLen(2))
			Expect(pods.Items[0].Spec.Containers[0].Name).To(Equal("route-registrar-route-registrar"))
		})
	})

	Context("when updating deployment", func() {
		var tearDowns []environment.TearDownFunc

		BeforeEach(func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			tearDowns = append(tearDowns, tearDown)
		})

		AfterEach(func() {
			Expect(env.TearDownAll(tearDowns)).To(Succeed())
		})

		Context("which has no ops files", func() {
			BeforeEach(func() {
				_, tearDown, err := env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test", "manifest"))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				By("checking for instance group pods")
				err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "1", 2)
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
			})

			Context("when updating the bdm custom resource with an ops file", func() {
				It("should update the deployment", func() {
					tearDown, err := env.CreateConfigMap(env.Namespace, env.InterpolateOpsConfigMap("test-ops"))
					Expect(err).NotTo(HaveOccurred())
					tearDowns = append(tearDowns, tearDown)

					bdm, err := env.GetBOSHDeployment(env.Namespace, "test")
					Expect(err).NotTo(HaveOccurred())
					bdm.Spec.Ops = []bdv1.Ops{{Ref: "test-ops", Type: bdv1.ConfigMapType}}
					_, _, err = env.UpdateBOSHDeployment(env.Namespace, *bdm)
					Expect(err).NotTo(HaveOccurred())

					By("checking for instance group updated pods")
					err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "2", 1)
					Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
				})
			})

			Context("when updating referenced BOSH deployment manifest", func() {
				It("should update the deployment", func() {
					cm, err := env.GetConfigMap(env.Namespace, "manifest")
					Expect(err).NotTo(HaveOccurred())
					cm.Data["manifest"] = strings.Replace(cm.Data["manifest"], "changeme", "dont", -1)
					_, _, err = env.UpdateConfigMap(env.Namespace, *cm)
					Expect(err).NotTo(HaveOccurred())

					By("checking for instance group updated pods")
					err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "2", 2)
					Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
				})
			})
		})

		Context("when updating referenced ops files", func() {
			BeforeEach(func() {
				tearDown, err := env.CreateConfigMap(env.Namespace, env.InterpolateOpsConfigMap("bosh-ops"))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeploymentWithOps("test", "manifest", "bosh-ops"))
				Expect(err).NotTo(HaveOccurred())
				tearDowns = append(tearDowns, tearDown)

				By("checking for instance group pods")
				err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "1", 1)
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")
			})

			scaleDeployment := func(n string) {
				ops, err := env.GetConfigMap(env.Namespace, "bosh-ops")
				Expect(err).NotTo(HaveOccurred())
				ops.Data["ops"] = `- type: replace
  path: /instance_groups/name=nats?/instances
  value: ` + n
				_, _, err = env.UpdateConfigMap(env.Namespace, *ops)
				Expect(err).NotTo(HaveOccurred())
			}

			It("should update the deployment and respect the instance count", func() {
				scaleDeployment("2")

				By("checking for instance group updated pods")
				err := env.WaitForInstanceGroup(env.Namespace, "test", "nats", "2", 2)
				Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

				pods, _ := env.GetInstanceGroupPods(env.Namespace, "test", "nats", "2")
				Expect(len(pods.Items)).To(Equal(2))

				By("updating the deployment again")
				scaleDeployment("3")

				By("checking if the deployment was again updated")
				err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "3", 3)
				Expect(err).NotTo(HaveOccurred(), "error waiting for pod from deployment")

				pods, _ = env.GetInstanceGroupPods(env.Namespace, "test", "nats", "3")
				Expect(len(pods.Items)).To(Equal(3))
			})
		})
	})

	Context("when updating a deployemnt with multiple instance groups", func() {
		It("it should only update correctly and have correct secret versions in volume mounts", func() {

			tearDown, err := env.CreateConfigMap(env.Namespace, env.BOSHManifestConfigMapWithTwoInstanceGroups("fooconfigmap"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test", "fooconfigmap"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			By("checking for nats instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for route_registrar instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "route_registrar", "1", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("Updating the deployment")
			cm, err := env.GetConfigMap(env.Namespace, "fooconfigmap")
			Expect(err).NotTo(HaveOccurred())
			cm.Data["manifest"] = strings.Replace(cm.Data["manifest"], "changeme", "dont", -1)
			_, _, err = env.UpdateConfigMap(env.Namespace, *cm)
			Expect(err).NotTo(HaveOccurred())

			By("checking for updated nats instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "nats", "2", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("checking for updated route_registrar instance group pods")
			err = env.WaitForInstanceGroup(env.Namespace, "test", "route_registrar", "2", 2)
			Expect(err).NotTo(HaveOccurred(), "error waiting for instance group pods from deployment")

			By("Checking volume mounts with secret versions")
			pod, err := env.GetPod(env.Namespace, "test-nats-v2-1")
			Expect(pod.Spec.Volumes[4].Secret.SecretName).To(Equal("test.desired-manifest-v2"))
			Expect(pod.Spec.Volumes[5].Secret.SecretName).To(Equal("test.ig-resolved.nats-v2"))
			Expect(pod.Spec.InitContainers[1].VolumeMounts[2].Name).To(Equal("ig-resolved"))

			pod, err = env.GetPod(env.Namespace, "test-route-registrar-v2-0")
			Expect(pod.Spec.Volumes[4].Secret.SecretName).To(Equal("test.desired-manifest-v2"))
			Expect(pod.Spec.Volumes[5].Secret.SecretName).To(Equal("test.ig-resolved.route-registrar-v2"))
			Expect(pod.Spec.InitContainers[1].VolumeMounts[2].Name).To(Equal("ig-resolved"))
		})
	})

	Context("when using a custom reconciler configuration", func() {
		It("should use the context timeout (1ns)", func() {
			env.Config.CtxTimeOut = 1 * time.Nanosecond
			defer func() {
				env.Config.CtxTimeOut = 10 * time.Second
			}()

			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test", "manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			Expect(env.WaitForLogMsg(env.ObservedLogs, "context deadline exceeded")).To(Succeed())
		})
	})

	Context("when data provided by the user is incorrect", func() {
		It("fails to create the resource if the validator gets an error when applying ops files", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			tearDown, err = env.CreateConfigMap(env.Namespace, env.InterpolateOpsConfigMap("bosh-ops"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			tearDown, err = env.CreateSecret(env.Namespace, env.InterpolateOpsIncorrectSecret("bosh-ops-secret"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.InterpolateBOSHDeployment("test", "manifest", "bosh-ops", "bosh-ops-secret"))
			Expect(err).To(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
			Expect(err.Error()).To(ContainSubstring(`admission webhook "validate-boshdeployment.fissile.cloudfoundry.org" denied the request:`))
		})

		It("failed to deploy an empty manifest", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.EmptyBOSHDeployment("test", "manifest"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.manifest.type in body should be one of"))
			Expect(err.Error()).To(ContainSubstring("spec.manifest.ref in body should be at least 1 chars long"))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
		})

		It("failed to deploy due to a wrong manifest type", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.WrongTypeBOSHDeployment("test", "manifest"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.manifest.type in body should be one of"))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
		})

		It("failed to deploy due to an empty manifest ref", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test", ""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.manifest.ref in body should be at least 1 chars long"))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
		})

		It("failed to deploy due to a wrong ops type", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.BOSHDeploymentWithWrongTypeOps("test", "manifest", "ops"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.ops.type in body should be one of"))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
		})

		It("failed to deploy due to an empty ops ref", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeploymentWithOps("test", "manifest", ""))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.ops.ref in body should be at least 1 chars long"))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
		})

		It("failed to deploy due to a not existing ops ref", func() {
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			// use a not created configmap name, so that we will hit errors while resources do not exist.
			_, tearDown, err = env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeploymentWithOps("test", "manifest", "bosh-ops-unknown"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Timeout reached. Resource bosh-ops-unknown does not exist"))
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)
		})

		It("failed to deploy if the ops resource is not available before timeout", func(done Done) {
			ch := make(chan environment.ChanResult)

			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			go env.CreateBOSHDeploymentUsingChan(ch, env.Namespace, env.DefaultBOSHDeploymentWithOps("test", "manifest", "bosh-ops"))

			time.Sleep(8 * time.Second)

			// Generate the right ops resource, so that the above goroutine will not end in error
			tearDown, err = env.CreateConfigMap(env.Namespace, env.InterpolateOpsConfigMap("bosh-ops"))
			Expect(err).NotTo(HaveOccurred())

			chanReceived := <-ch
			Expect(chanReceived.Error).To(HaveOccurred())
			close(done)
		}, 10)

		It("does not failed to deploy if the ops ref is created on time", func(done Done) {
			ch := make(chan environment.ChanResult)
			tearDown, err := env.CreateConfigMap(env.Namespace, env.DefaultBOSHManifestConfigMap("manifest"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			go env.CreateBOSHDeploymentUsingChan(ch, env.Namespace, env.DefaultBOSHDeploymentWithOps("test", "manifest", "bosh-ops"))

			// Generate the right ops resource, so that the above goroutine will not end in error
			tearDown, err = env.CreateConfigMap(env.Namespace, env.InterpolateOpsConfigMap("bosh-ops"))
			Expect(err).NotTo(HaveOccurred())
			defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(tearDown)

			chanReceived := <-ch
			Expect(chanReceived.Error).NotTo(HaveOccurred())
			close(done)
		}, 5)
	})

	Context("when the BOSHDeployment cannot be resolved", func() {
		It("should not create the resource and the validation hook should return an error message", func() {
			_, _, err := env.CreateBOSHDeployment(env.Namespace, env.DefaultBOSHDeployment("test", "foo-baz"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`admission webhook "validate-boshdeployment.fissile.cloudfoundry.org" denied the request:`))
			Expect(err.Error()).To(ContainSubstring(`ConfigMap "foo-baz" not found`))
		})
	})
})
