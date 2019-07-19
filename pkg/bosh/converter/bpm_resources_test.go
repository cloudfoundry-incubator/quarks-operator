package converter_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/testing"
	"code.cloudfoundry.org/cf-operator/testing/boshreleases"
)

var _ = Describe("kube converter", func() {
	var (
		m   manifest.Manifest
		env testing.Catalog
	)

	Context("BPMResources", func() {
		act := func(bpmConfigs bpm.Configs, instanceGroup *manifest.InstanceGroup) (*converter.BPMResources, error) {
			kubeConverter := converter.NewKubeConverter("foo")
			resources, err := kubeConverter.BPMResources(m.Name, "1", instanceGroup, &m, bpmConfigs)
			return resources, err
		}

		BeforeEach(func() {
			m = env.DefaultBOSHManifest()
		})

		Context("when BPM is missing in configs", func() {
			It("returns an error", func() {
				bpmConfigs := bpm.Configs{}
				_, err := act(bpmConfigs, m.InstanceGroups[0])
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to lookup bpm config for bosh job 'redis-server' in bpm configs"))
			})
		})

		Context("when a BPM config is present", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				c, err := bpm.NewConfig([]byte(boshreleases.DefaultBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"redis-server": c},
					{"cflinuxfs3-rootfs-setup": c},
				}

			})

			Context("when the lifecycle is set to errand", func() {
				It("converts the instance group to an ExtendedJob", func() {
					kubeConverter, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(kubeConverter.Errands).To(HaveLen(1))

					// Test labels and annotations in the extended job
					eJob := kubeConverter.Errands[0]
					Expect(eJob.Name).To(Equal("foo-deployment-redis-slave"))
					Expect(eJob.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentName, m.Name))
					Expect(eJob.GetLabels()).To(HaveKeyWithValue(manifest.LabelInstanceGroupName, m.InstanceGroups[0].Name))
					Expect(eJob.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentVersion, "1"))
					Expect(eJob.GetLabels()).To(HaveKeyWithValue("custom-label", "foo"))
					Expect(eJob.GetAnnotations()).To(HaveKeyWithValue("custom-annotation", "bar"))

					specCopierInitContainer := eJob.Spec.Template.Spec.InitContainers[0]
					rendererInitContainer := eJob.Spec.Template.Spec.InitContainers[1]

					// Test containers in the extended job
					Expect(eJob.Spec.Template.Spec.Containers[0].Name).To(Equal("redis-server-test-server"))
					Expect(eJob.Spec.Template.Spec.Containers[0].Image).To(Equal("hub.docker.com/cfcontainerization/redis:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
					Expect(eJob.Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"/var/vcap/packages/test-server/bin/test-server"}))
					Expect(eJob.Spec.Template.Spec.Containers[0].VolumeMounts[4].Name).To(Equal("bpm-additional-volume-redis-server-test-server-0"))
					Expect(eJob.Spec.Template.Spec.Containers[0].VolumeMounts[5].Name).To(Equal("bpm-additional-volume-redis-server-test-server-1"))
					Expect(eJob.Spec.Template.Spec.Containers[0].VolumeMounts[6].Name).To(Equal("bpm-unrestricted-volume-redis-server-test-server-0"))
					Expect(eJob.Spec.Template.Spec.Containers[0].VolumeMounts[7].Name).To(Equal("bpm-ephemeral-disk"))

					// Test init containers in the extended job
					Expect(specCopierInitContainer.Name).To(Equal("spec-copier-redis"))
					Expect(specCopierInitContainer.Image).To(Equal("hub.docker.com/cfcontainerization/redis:opensuse-42.3-28.g837c5b3-30.263-7.0.0_234.gcd7d1132-36.15.0"))
					Expect(specCopierInitContainer.Command[0]).To(Equal("/bin/sh"))
					Expect(rendererInitContainer.Image).To(Equal("/:"))
					Expect(rendererInitContainer.Name).To(Equal("template-render"))

					// Test shared volume setup
					Expect(eJob.Spec.Template.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("rendering-data"))
					Expect(eJob.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
					Expect(specCopierInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
					Expect(specCopierInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
					Expect(rendererInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
					Expect(rendererInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))

					// Test mounting the resolved instance group properties in the renderer container
					Expect(rendererInitContainer.Env[0].Name).To(Equal("INSTANCE_GROUP_NAME"))
					Expect(rendererInitContainer.Env[0].Value).To(Equal("redis-slave"))
					Expect(rendererInitContainer.VolumeMounts[1].Name).To(Equal("jobs-dir"))
					Expect(rendererInitContainer.VolumeMounts[1].MountPath).To(Equal("/var/vcap/jobs"))

					// Test affinity
					Expect(eJob.Spec.Template.Spec.Affinity).To(BeNil())
				})
			})

			Context("when the lifecycle is set to service", func() {
				It("converts the instance group to an ExtendedStatefulSet", func() {
					resources, err := act(bpmConfigs[1], m.InstanceGroups[1])
					Expect(err).ShouldNot(HaveOccurred())

					// Test labels and annotation in the extended statefulSet
					extStS := resources.InstanceGroups[0]
					Expect(extStS.Name).To(Equal(fmt.Sprintf("%s-%s", m.Name, "diego-cell")))
					Expect(extStS.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentName, m.Name))
					Expect(extStS.GetLabels()).To(HaveKeyWithValue(manifest.LabelInstanceGroupName, "diego-cell"))
					Expect(extStS.GetLabels()).To(HaveKeyWithValue(manifest.LabelDeploymentVersion, "1"))

					stS := extStS.Spec.Template.Spec.Template
					Expect(stS.Name).To(Equal("diego-cell"))

					specCopierInitContainer := stS.Spec.InitContainers[0]
					rendererInitContainer := stS.Spec.InitContainers[1]

					// Test containers in the extended statefulSet
					Expect(stS.Spec.Containers[0].Image).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
					Expect(stS.Spec.Containers[0].Command).To(Equal([]string{"/var/vcap/packages/test-server/bin/test-server"}))
					Expect(stS.Spec.Containers[0].Name).To(Equal("cflinuxfs3-rootfs-setup-test-server"))

					// Test init containers in the extended statefulSet
					Expect(specCopierInitContainer.Name).To(Equal("spec-copier-cflinuxfs3"))
					Expect(specCopierInitContainer.Image).To(Equal("hub.docker.com/cfcontainerization/cflinuxfs3:opensuse-15.0-28.g837c5b3-30.263-7.0.0_233.gde0accd0-0.62.0"))
					Expect(specCopierInitContainer.Command[0]).To(Equal("/bin/sh"))
					Expect(specCopierInitContainer.Name).To(Equal("spec-copier-cflinuxfs3"))
					Expect(rendererInitContainer.Image).To(Equal("/:"))
					Expect(rendererInitContainer.Name).To(Equal("template-render"))

					// Test shared volume setup
					Expect(len(stS.Spec.Containers[0].VolumeMounts)).To(Equal(8))
					Expect(stS.Spec.Containers[0].VolumeMounts[0].Name).To(Equal("rendering-data"))
					Expect(stS.Spec.Containers[0].VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
					Expect(stS.Spec.Containers[0].VolumeMounts[4].Name).To(Equal("bpm-additional-volume-cflinuxfs3-rootfs-setup-test-server-0"))
					Expect(stS.Spec.Containers[0].VolumeMounts[4].MountPath).To(Equal("/var/vcap/data/shared"))
					Expect(stS.Spec.Containers[0].VolumeMounts[4].ReadOnly).To(Equal(false))
					Expect(stS.Spec.Containers[0].VolumeMounts[5].Name).To(Equal("bpm-additional-volume-cflinuxfs3-rootfs-setup-test-server-1"))
					Expect(stS.Spec.Containers[0].VolumeMounts[5].MountPath).To(Equal("/var/vcap/store/foo"))
					Expect(stS.Spec.Containers[0].VolumeMounts[5].ReadOnly).To(Equal(false))
					Expect(stS.Spec.Containers[0].VolumeMounts[6].Name).To(Equal("bpm-unrestricted-volume-cflinuxfs3-rootfs-setup-test-server-0"))
					Expect(stS.Spec.Containers[0].VolumeMounts[6].MountPath).To(Equal("/dev/log"))
					Expect(stS.Spec.Containers[0].VolumeMounts[7].Name).To(Equal("bpm-ephemeral-disk"))
					Expect(stS.Spec.Containers[0].VolumeMounts[7].MountPath).To(Equal("/var/vcap/data/cflinuxfs3-rootfs-setup"))
					Expect(specCopierInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
					Expect(specCopierInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
					Expect(rendererInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
					Expect(rendererInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))

					// Test share pod spec volumes
					Expect(len(stS.Spec.Volumes)).To(Equal(10))

					Expect(stS.Spec.Volumes[6].Name).To(Equal("bpm-additional-volume-cflinuxfs3-rootfs-setup-test-server-0"))
					Expect(stS.Spec.Volumes[6].EmptyDir).To(Equal(&corev1.EmptyDirVolumeSource{}))

					Expect(stS.Spec.Volumes[7].Name).To(Equal("bpm-additional-volume-cflinuxfs3-rootfs-setup-test-server-1"))
					Expect(stS.Spec.Volumes[7].EmptyDir).To(Equal(&corev1.EmptyDirVolumeSource{}))

					Expect(stS.Spec.Volumes[8].Name).To(Equal("bpm-unrestricted-volume-cflinuxfs3-rootfs-setup-test-server-0"))
					Expect(stS.Spec.Volumes[8].EmptyDir).To(Equal(&corev1.EmptyDirVolumeSource{}))

					Expect(stS.Spec.Volumes[9].Name).To(Equal("bpm-ephemeral-disk"))
					Expect(stS.Spec.Volumes[9].EmptyDir).To(Equal(&corev1.EmptyDirVolumeSource{}))

					// Test the renderer container setup
					Expect(rendererInitContainer.Env[0].Name).To(Equal("INSTANCE_GROUP_NAME"))
					Expect(rendererInitContainer.Env[0].Value).To(Equal("diego-cell"))
					Expect(rendererInitContainer.VolumeMounts[0].Name).To(Equal("rendering-data"))
					Expect(rendererInitContainer.VolumeMounts[0].MountPath).To(Equal("/var/vcap/all-releases"))
					Expect(rendererInitContainer.VolumeMounts[1].Name).To(Equal("jobs-dir"))
					Expect(rendererInitContainer.VolumeMounts[1].MountPath).To(Equal("/var/vcap/jobs"))
					Expect(rendererInitContainer.VolumeMounts[2].Name).To(Equal("ig-resolved"))
					Expect(rendererInitContainer.VolumeMounts[2].MountPath).To(Equal("/var/run/secrets/resolved-properties/diego-cell"))

					// Test the healthcheck setup
					readinessProbe := stS.Spec.Containers[0].ReadinessProbe
					Expect(readinessProbe).ToNot(BeNil())
					Expect(readinessProbe.Exec.Command[0]).To(Equal("curl --silent --fail --head http://${HOSTNAME}:8080/health"))

					livenessProbe := stS.Spec.Containers[0].LivenessProbe
					Expect(livenessProbe).ToNot(BeNil())
					Expect(livenessProbe.Exec.Command[0]).To(Equal("curl --silent --fail --head http://${HOSTNAME}:8080"))

					// Test services for the extended statefulSet
					service0 := resources.Services[0]
					Expect(service0.Name).To(Equal(fmt.Sprintf("%s-%s-0", m.Name, stS.Name)))
					Expect(service0.Spec.Selector).To(Equal(map[string]string{
						manifest.LabelInstanceGroupName: stS.Name,
						essv1.LabelAZIndex:              "0",
						essv1.LabelPodOrdinal:           "0",
					}))
					Expect(service0.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					service1 := resources.Services[1]
					Expect(service1.Name).To(Equal(fmt.Sprintf("%s-%s-1", m.Name, stS.Name)))
					Expect(service1.Spec.Selector).To(Equal(map[string]string{
						manifest.LabelInstanceGroupName: stS.Name,
						essv1.LabelAZIndex:              "1",
						essv1.LabelPodOrdinal:           "0",
					}))
					Expect(service1.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					service2 := resources.Services[2]
					Expect(service2.Name).To(Equal(fmt.Sprintf("%s-%s-2", m.Name, stS.Name)))
					Expect(service2.Spec.Selector).To(Equal(map[string]string{
						manifest.LabelInstanceGroupName: stS.Name,
						essv1.LabelAZIndex:              "0",
						essv1.LabelPodOrdinal:           "1",
					}))
					Expect(service2.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					service3 := resources.Services[3]
					Expect(service3.Name).To(Equal(fmt.Sprintf("%s-%s-3", m.Name, stS.Name)))
					Expect(service3.Spec.Selector).To(Equal(map[string]string{
						manifest.LabelInstanceGroupName: stS.Name,
						essv1.LabelAZIndex:              "1",
						essv1.LabelPodOrdinal:           "1",
					}))
					Expect(service3.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					headlessService := resources.Services[4]
					Expect(headlessService.Name).To(Equal(fmt.Sprintf("%s-%s", m.Name, stS.Name)))
					Expect(headlessService.Spec.Selector).To(Equal(map[string]string{
						manifest.LabelInstanceGroupName: stS.Name,
					}))
					Expect(headlessService.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))
					Expect(headlessService.Spec.ClusterIP).To(Equal("None"))

					// Test affinity
					Expect(stS.Spec.Affinity).To(BeNil())
				})
			})
		})

		Context("when multiple BPM processes exist", func() {
			var (
				bpmConfigs []bpm.Configs
				bpmConfig1 bpm.Config
				bpmConfig2 bpm.Config
			)

			BeforeEach(func() {
				var err error
				m = *env.BOSHManifestWithMultiBPMProcesses()

				bpmConfig1, err = bpm.NewConfig([]byte(boshreleases.DefaultBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())
				bpmConfig2, err = bpm.NewConfig([]byte(boshreleases.MultiProcessBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{
						"fake-errand-a": bpmConfig1,
						"fake-errand-b": bpmConfig2,
					},
					{
						"fake-job-a": bpmConfig1,
						"fake-job-b": bpmConfig1,
						"fake-job-c": bpmConfig2,
					},
					{
						"fake-job-a": bpmConfig1,
						"fake-job-b": bpmConfig1,
						"fake-job-c": bpmConfig2,
						"fake-job-d": bpmConfig2,
					},
				}
			})

			It("creates a k8s container for each BPM process", func() {
				resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
				Expect(err).ShouldNot(HaveOccurred())
				Expect(resources.Errands).To(HaveLen(1))
				containers := resources.Errands[0].Spec.Template.Spec.Containers
				Expect(containers).To(HaveLen(4))
				Expect(containers[0].Name).To(Equal("fake-errand-a-test-server"))
				Expect(containers[0].Command[0]).To(ContainSubstring("bin/test-server"))
				Expect(containers[0].Args).To(HaveLen(2))
				Expect(containers[0].Args[0]).To(Equal("--port"))
				Expect(containers[0].Args[1]).To(Equal("1337"))
				Expect(containers[0].Env).To(HaveLen(1))
				Expect(containers[0].Env[0].Name).To(Equal("BPM"))
				Expect(containers[0].Env[0].Value).To(Equal("SWEET"))
				Expect(containers[1].Name).To(Equal("fake-errand-b-test-server"))
				Expect(containers[2].Name).To(Equal("fake-errand-b-alt-test-server"))
				Expect(containers[2].Command[0]).To(ContainSubstring("bin/test-server"))
				Expect(containers[2].Args).To(HaveLen(3))
				Expect(containers[2].Args[0]).To(Equal("--port"))
				Expect(containers[2].Args[1]).To(Equal("1338"))
				Expect(containers[2].Env).To(HaveLen(1))
				Expect(containers[2].Env[0].Name).To(Equal("BPM"))
				Expect(containers[2].Env[0].Value).To(Equal("CONTAINED"))

				resources, err = act(bpmConfigs[1], m.InstanceGroups[1])
				Expect(err).ShouldNot(HaveOccurred())
				containers = resources.InstanceGroups[0].Spec.Template.Spec.Template.Spec.Containers
				Expect(containers).To(HaveLen(5))
				Expect(containers[0].Name).To(Equal("fake-job-a-test-server"))
				Expect(containers[1].Name).To(Equal("fake-job-b-test-server"))
				Expect(containers[2].Name).To(Equal("fake-job-c-test-server"))
				Expect(containers[3].Name).To(Equal("fake-job-c-alt-test-server"))

				resources, err = act(bpmConfigs[2], m.InstanceGroups[2])
				Expect(err).ShouldNot(HaveOccurred())
				Expect(resources.InstanceGroups).To(HaveLen(1))
				containers = resources.InstanceGroups[0].Spec.Template.Spec.Template.Spec.Containers
				Expect(containers).To(HaveLen(7))
				Expect(containers[0].Name).To(Equal("fake-job-a-test-server"))
				Expect(containers[1].Name).To(Equal("fake-job-b-test-server"))
				Expect(containers[2].Name).To(Equal("fake-job-c-test-server"))
				Expect(containers[3].Name).To(Equal("fake-job-c-alt-test-server"))
				Expect(containers[4].Name).To(Equal("fake-job-d-test-server"))
				Expect(containers[5].Name).To(Equal("fake-job-d-alt-test-server"))
			})
		})

		Context("when the instance group name contains an underscore", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m = *env.BOSHManifestCFRouting()

				c, err := bpm.NewConfig([]byte(boshreleases.CFRouting))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"route_registrar": c},
				}

			})

			Context("when the lifecycle is set to errand", func() {
				It("converts the instance group to an ExtendedJob", func() {
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.InstanceGroups).To(HaveLen(1))
				})
			})
		})

		Context("when the instance group contains a persistent disk declaration", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m = *env.BOSHManifestWithBPMRelease()

				c, err := bpm.NewConfig([]byte(boshreleases.EnablePersistentDiskBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"test-server": c},
				}

			})

			Context("when the lifecycle is set to service", func() {
				It("converts the disks and volume declarations", func() {
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())

					extStS := resources.InstanceGroups[0]
					stS := extStS.Spec.Template.Spec.Template

					// Test shared volume setup
					Expect(len(stS.Spec.Containers[0].VolumeMounts)).To(Equal(9))
					Expect(stS.Spec.Containers[0].VolumeMounts[8].Name).To(Equal("store-dir"))
					Expect(stS.Spec.Containers[0].VolumeMounts[8].MountPath).To(Equal("/var/vcap/store/test-server"))
					Expect(stS.Spec.Containers[0].VolumeMounts[8].SubPath).To(Equal("test-server"))

					// Test share pod spec volumes
					Expect(len(stS.Spec.Volumes)).To(Equal(11))
					Expect(stS.Spec.Volumes[10].Name).To(Equal("store-dir"))
					Expect(stS.Spec.Volumes[10].PersistentVolumeClaim.ClaimName).To(Equal("bpm-bpm-pvc"))

					// Test disks
					disks := resources.Disks
					Expect(disks[10].PersistentVolumeClaim).NotTo(Equal(nil))
					Expect(disks[10].VolumeMount.Name).To(Equal("store-dir"))
					Expect(disks[10].VolumeMount.MountPath).To(Equal("/var/vcap/store/test-server"))

					persistentDisks := disks.Filter("persistent", "true")
					Expect(persistentDisks[0].VolumeMount.Name).To(Equal("store-dir"))
					Expect(persistentDisks[0].VolumeMount.MountPath).To(Equal("/var/vcap/store/test-server"))
					Expect(persistentDisks[0].VolumeMount.SubPath).To(Equal("test-server"))
				})
			})
		})

		Context("when the job contains a persistent disk declaration", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m = *env.BOSHManifestWithBPMRelease()

				c, err := bpm.NewConfig([]byte(boshreleases.EnablePersistentDiskBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"test-server": c},
				}

			})

			It("converts the disks and volume declarations when instance group has persistent disk declaration", func() {
				resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
				Expect(err).ShouldNot(HaveOccurred())

				extStS := resources.InstanceGroups[0]
				stS := extStS.Spec.Template.Spec.Template

				// Test shared volume setup
				Expect(len(stS.Spec.Containers[0].VolumeMounts)).To(Equal(9))
				Expect(stS.Spec.Containers[0].VolumeMounts[8].Name).To(Equal("store-dir"))
				Expect(stS.Spec.Containers[0].VolumeMounts[8].MountPath).To(Equal("/var/vcap/store/test-server"))
				Expect(stS.Spec.Containers[0].VolumeMounts[8].SubPath).To(Equal("test-server"))

				// Test share pod spec volumes
				Expect(len(stS.Spec.Volumes)).To(Equal(11))
				Expect(stS.Spec.Volumes[10].Name).To(Equal("store-dir"))
				Expect(stS.Spec.Volumes[10].PersistentVolumeClaim.ClaimName).To(Equal("bpm-bpm-pvc"))

				// Test disks
				disks := resources.Disks
				Expect(disks[10].PersistentVolumeClaim).NotTo(Equal(nil))
				Expect(disks[10].VolumeMount.Name).To(Equal("store-dir"))
				Expect(disks[10].VolumeMount.MountPath).To(Equal("/var/vcap/store/test-server"))

				persistentDisks := disks.Filter("persistent", "true")
				Expect(persistentDisks[0].VolumeMount.Name).To(Equal("store-dir"))
				Expect(persistentDisks[0].VolumeMount.MountPath).To(Equal("/var/vcap/store/test-server"))
				Expect(persistentDisks[0].VolumeMount.SubPath).To(Equal("test-server"))
			})

			It("handles error when instance group doesn't have persistent disk declaration", func() {
				m = *env.BOSHManifestWithoutPersistentDisk()

				_, err := act(bpmConfigs[0], m.InstanceGroups[0])
				Expect(err).Should(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("job '%s' wants to use persistent disk"+
					" but instance group '%s' doesn't have any persistent disk declaration", "test-server", "bpm")))
			})

			Context("when multiple BPM processes exist", func() {
				var (
					bpmConfigs []bpm.Configs
					bpmConfig1 bpm.Config
					bpmConfig2 bpm.Config
				)

				BeforeEach(func() {
					var err error
					m = *env.BOSHManifestWithMultiBPMProcessesAndPersistentDisk()

					bpmConfig1, err = bpm.NewConfig([]byte(boshreleases.DefaultBPMConfig))
					Expect(err).ShouldNot(HaveOccurred())
					bpmConfig2, err = bpm.NewConfig([]byte(boshreleases.MultiProcessBPMConfigWithPersistentDisk))
					Expect(err).ShouldNot(HaveOccurred())

					bpmConfigs = []bpm.Configs{
						{
							"fake-errand-a": bpmConfig1,
							"fake-errand-b": bpmConfig2,
						},
						{
							"fake-job-a": bpmConfig1,
							"fake-job-b": bpmConfig1,
							"fake-job-c": bpmConfig2,
						},
						{
							"fake-job-a": bpmConfig1,
							"fake-job-b": bpmConfig1,
							"fake-job-c": bpmConfig2,
							"fake-job-d": bpmConfig2,
						},
					}
				})

				It("converts correct disks and volume declarations", func() {
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.Errands).To(HaveLen(1))
					containers := resources.Errands[0].Spec.Template.Spec.Containers

					// Test shared volume setup
					Expect(containers[1].VolumeMounts[5].Name).To(Equal("bpm-ephemeral-disk"))
					Expect(containers[1].VolumeMounts[5].MountPath).To(Equal("/var/vcap/data/fake-errand-b"))
					Expect(containers[1].VolumeMounts[6].Name).To(Equal("store-dir"))
					Expect(containers[1].VolumeMounts[6].MountPath).To(Equal("/var/vcap/store/fake-errand-b"))
					Expect(containers[1].VolumeMounts[6].SubPath).To(Equal("fake-errand-b"))
				})
			})
		})

		Context("when affinity is provided", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m = *env.BPMReleaseWithAffinity()

				c, err := bpm.NewConfig([]byte(boshreleases.DefaultBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"test-server": c},
				}

			})

			It("adds affinity into the pod's definition", func() {
				r1, err := act(bpmConfigs[0], m.InstanceGroups[0])
				Expect(err).ShouldNot(HaveOccurred())

				// Test node affinity
				ig1 := r1.InstanceGroups[0]
				Expect(ig1.Spec.Template.Spec.Template.Spec.Affinity.NodeAffinity).To(Equal(&corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "beta.kubernetes.io/os",
										Operator: "In",
										Values: []string{
											"linux",
											"darwin",
										},
									},
								},
							},
						},
					},
				}))

				r2, err := act(bpmConfigs[0], m.InstanceGroups[1])
				Expect(err).ShouldNot(HaveOccurred())

				// Test pod affinity
				ig2 := r2.InstanceGroups[0]
				Expect(ig2.Spec.Template.Spec.Template.Spec.Affinity.PodAffinity).To(Equal(&corev1.PodAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
						{
							LabelSelector: &metav1.LabelSelector{
								MatchExpressions: []metav1.LabelSelectorRequirement{
									{
										Key:      "instance-name",
										Operator: "In",
										Values: []string{
											"bpm2",
										},
									},
								},
							},
							TopologyKey: "beta.kubernetes.io/os",
						},
					},
				}))

				r3, err := act(bpmConfigs[0], m.InstanceGroups[2])
				Expect(err).ShouldNot(HaveOccurred())

				// Test pod anti-affinity
				ig3 := r3.InstanceGroups[0]
				Expect(ig3.Spec.Template.Spec.Template.Spec.Affinity.PodAntiAffinity).To(Equal(&corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 100,
							PodAffinityTerm: corev1.PodAffinityTerm{
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "instance-name",
											Operator: "In",
											Values: []string{
												"bpm3",
											},
										},
									},
								},
								TopologyKey: "beta.kubernetes.io/os",
							},
						},
					},
				}))
			})
		})
	})
})
