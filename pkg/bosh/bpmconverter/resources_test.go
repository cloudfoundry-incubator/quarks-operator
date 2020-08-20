package bpmconverter_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpmconverter"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpmconverter/fakes"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/testing"
	"code.cloudfoundry.org/quarks-operator/testing/boshreleases"
	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-statefulset/pkg/kube/controllers/statefulset"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

var _ = Describe("BPM Converter", func() {
	var (
		m                *manifest.Manifest
		deploymentName   string
		volumeFactory    *fakes.FakeVolumeFactory
		containerFactory *fakes.FakeContainerFactory
		env              testing.Catalog
		err              error
	)

	Context("Resources", func() {
		act := func(bpmConfigs bpm.Configs, instanceGroup *manifest.InstanceGroup) (*bpmconverter.Resources, error) {
			c := bpmconverter.NewConverter(
				volumeFactory,
				func(instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider manifest.ReleaseImageProvider, bpmConfigs bpm.Configs) bpmconverter.ContainerFactory {
					return containerFactory
				})
			resources, err := c.Resources(*m, "foo", deploymentName, "1.2.3.4", "1", instanceGroup, bpmConfigs, "1")
			return resources, err
		}

		BeforeEach(func() {
			deploymentName = "fake-deployment"

			m, err = env.DefaultBOSHManifest()
			Expect(err).NotTo(HaveOccurred())

			volumeFactory = &fakes.FakeVolumeFactory{}
			containerFactory = &fakes.FakeContainerFactory{}
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
				It("handles an error when generating bpm disks", func() {
					volumeFactory.GenerateBPMDisksReturns(manifest.Disks{}, errors.New("fake-bpm-disk-error"))
					_, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Generate of BPM disks failed for manifest name %s, instance group %s.", deploymentName, m.InstanceGroups[0].Name))
				})

				It("handles an error when converting jobs to initContainers", func() {
					containerFactory.JobsToInitContainersReturns([]corev1.Container{}, errors.New("fake-container-factory-error"))
					_, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("building initContainers failed for instance group %s", m.InstanceGroups[0].Name))
				})

				It("handles an error when converting jobs to containers", func() {
					containerFactory.JobsToContainersReturns([]corev1.Container{}, errors.New("fake-container-factory-error"))
					_, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("building containers failed for instance group %s", m.InstanceGroups[0].Name))
				})

				It("converts the instance group to an quarksJob", func() {
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.Errands).To(HaveLen(1))

					// Test labels and annotations in the quarks job
					qJob := resources.Errands[0]
					Expect(qJob.Name).To(Equal("redis-slave"))
					Expect(qJob.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentName, deploymentName))
					Expect(qJob.GetLabels()).To(HaveKeyWithValue(bdv1.LabelInstanceGroupName, m.InstanceGroups[0].Name))
					Expect(qJob.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentVersion, "1"))
					Expect(qJob.GetLabels()).To(HaveKeyWithValue("custom-label", "foo"))
					Expect(qJob.GetAnnotations()).To(HaveKeyWithValue("custom-annotation", "bar"))

					// Test affinity & tolerations
					Expect(qJob.Spec.Template.Spec.Template.Spec.Affinity).To(BeNil())
					Expect(len(qJob.Spec.Template.Spec.Template.Spec.Tolerations)).To(Equal(0))

					Expect(qJob.Spec.Template.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
				})

				It("converts the instance group to an quarksJob when this the lifecycle is set to auto-errand", func() {
					m.InstanceGroups[0].LifeCycle = manifest.IGTypeAutoErrand
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.Errands).To(HaveLen(1))

					// Test trigger strategy
					qJob := resources.Errands[0]
					Expect(qJob.Spec.Trigger.Strategy).To(Equal(qjv1a1.TriggerOnce))

					Expect(qJob.Spec.Template.Spec.Template.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
				})

				It("converts the AgentEnvBoshConfig information", func() {
					affinityCase := corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "fake-key",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"fake-label"},
											},
										},
									},
								},
							},
						},
					}
					tolerations := []corev1.Toleration{
						{
							Key:      "key",
							Operator: "Equal",
							Value:    "value",
							Effect:   "NoSchedule",
						},
					}
					serviceAccount := "fake-service-account"
					automountServiceAccountToken := true
					m.InstanceGroups[0].Env.AgentEnvBoshConfig.Agent.Settings.Affinity = &affinityCase
					m.InstanceGroups[0].Env.AgentEnvBoshConfig.Agent.Settings.Tolerations = tolerations
					m.InstanceGroups[0].Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName = serviceAccount
					m.InstanceGroups[0].Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken = &automountServiceAccountToken
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.Errands).To(HaveLen(1))

					// Test AgentEnvBoshConfig
					qJob := resources.Errands[0]
					Expect(*qJob.Spec.Template.Spec.Template.Spec.Affinity).To(Equal(affinityCase))
					Expect(qJob.Spec.Template.Spec.Template.Spec.Tolerations).To(Equal(tolerations))
					Expect(qJob.Spec.Template.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))
					Expect(*qJob.Spec.Template.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(automountServiceAccountToken))
				})
			})

			Context("when the lifecycle is set to service", func() {
				It("handles an error when converting jobs to initContainers", func() {
					containerFactory.JobsToInitContainersReturns([]corev1.Container{}, errors.New("fake-container-factory-error"))
					_, err := act(bpmConfigs[1], m.InstanceGroups[1])
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("building initContainers failed for instance group %s", m.InstanceGroups[1].Name))
				})

				It("handles an error when converting jobs to containers", func() {
					containerFactory.JobsToContainersReturns([]corev1.Container{}, errors.New("fake-container-factory-error"))
					_, err := act(bpmConfigs[1], m.InstanceGroups[1])
					Expect(err).Should(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("building containers failed for instance group %s", m.InstanceGroups[1].Name))
				})

				It("converts the instance group to an QuarksStatefulSet", func() {

					tolerations := []corev1.Toleration{
						{
							Key:      "key",
							Operator: "Equal",
							Value:    "value",
							Effect:   "NoSchedule",
						},
					}
					m.InstanceGroups[1].Env.AgentEnvBoshConfig.Agent.Settings.Tolerations = tolerations

					activePassiveProbes := map[string]corev1.Probe{
						"rep-server": corev1.Probe{
							Handler: corev1.Handler{
								Exec: &corev1.ExecAction{
									Command: []string{"ls", "/"},
								},
							},
						},
					}
					config := bpmConfigs[1]["cflinuxfs3-rootfs-setup"]
					config.ActivePassiveProbes = activePassiveProbes
					config.Ports = []bpm.Port{
						{
							Name:     "rep-server",
							Protocol: "TCP",
							Internal: 1801,
						},
					}
					bpmConfigs[1]["cflinuxfs3-rootfs-setup"] = config

					resources, err := act(bpmConfigs[1], m.InstanceGroups[1])
					Expect(err).ShouldNot(HaveOccurred())

					qSts := resources.InstanceGroups[0]
					stS := qSts.Spec.Template.Spec.Template

					// Test services for the quarks statefulSet
					//
					service0 := resources.Services[0]
					Expect(service0.Spec.Selector).To(Equal(map[string]string{
						bdv1.LabelDeploymentName:    deploymentName,
						bdv1.LabelInstanceGroupName: stS.Name,
						qstsv1a1.LabelAZIndex:       "0",
						qstsv1a1.LabelPodOrdinal:    "0",
						qstsv1a1.LabelActivePod:     "active",
					}))
				})

				It("converts the instance group to an QuarksStatefulSet", func() {
					tolerations := []corev1.Toleration{
						{
							Key:      "key",
							Operator: "Equal",
							Value:    "value",
							Effect:   "NoSchedule",
						},
					}
					m.InstanceGroups[1].Env.AgentEnvBoshConfig.Agent.Settings.Tolerations = tolerations

					resources, err := act(bpmConfigs[1], m.InstanceGroups[1])
					Expect(err).ShouldNot(HaveOccurred())

					// Test labels and annotation in the quarks statefulSet
					qSts := resources.InstanceGroups[0]
					Expect(qSts.Name).To(Equal("diego-cell"))
					Expect(qSts.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentName, deploymentName))
					Expect(qSts.GetLabels()).To(HaveKeyWithValue(bdv1.LabelInstanceGroupName, "diego-cell"))
					Expect(qSts.GetLabels()).To(HaveKeyWithValue(bdv1.LabelDeploymentVersion, "1"))

					// Test ESts spec
					Expect(qSts.Spec.Zones).To(Equal(m.InstanceGroups[1].AZs))

					stS := qSts.Spec.Template.Spec.Template
					Expect(stS.Name).To(Equal("diego-cell"))

					// Test services for the quarks statefulSet
					service0 := resources.Services[0]
					Expect(service0.Name).To(Equal(fmt.Sprintf("%s-z%d-0", stS.Name, 0)))
					Expect(service0.Spec.Selector).To(Equal(map[string]string{
						bdv1.LabelDeploymentName:    deploymentName,
						bdv1.LabelInstanceGroupName: stS.Name,
						qstsv1a1.LabelAZIndex:       "0",
						qstsv1a1.LabelPodOrdinal:    "0",
					}))
					Expect(service0.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					service1 := resources.Services[1]
					Expect(service1.Name).To(Equal(fmt.Sprintf("%s-z%d-1", stS.Name, 0)))
					Expect(service1.Spec.Selector).To(Equal(map[string]string{
						bdv1.LabelDeploymentName:    deploymentName,
						bdv1.LabelInstanceGroupName: stS.Name,
						qstsv1a1.LabelAZIndex:       "0",
						qstsv1a1.LabelPodOrdinal:    "1",
					}))
					Expect(service1.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					service2 := resources.Services[2]
					Expect(service2.Name).To(Equal(fmt.Sprintf("%s-z%d-0", stS.Name, 1)))
					Expect(service2.Spec.Selector).To(Equal(map[string]string{
						bdv1.LabelDeploymentName:    deploymentName,
						bdv1.LabelInstanceGroupName: stS.Name,
						qstsv1a1.LabelAZIndex:       "1",
						qstsv1a1.LabelPodOrdinal:    "0",
					}))
					Expect(service2.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					service3 := resources.Services[3]
					Expect(service3.Name).To(Equal(fmt.Sprintf("%s-z%d-1", stS.Name, 1)))
					Expect(service3.Spec.Selector).To(Equal(map[string]string{
						bdv1.LabelDeploymentName:    deploymentName,
						bdv1.LabelInstanceGroupName: stS.Name,
						qstsv1a1.LabelAZIndex:       "1",
						qstsv1a1.LabelPodOrdinal:    "1",
					}))
					Expect(service3.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))

					headlessService := resources.Services[4]
					Expect(headlessService.Name).To(Equal(stS.Name))
					Expect(headlessService.Spec.Selector).To(Equal(map[string]string{
						bdv1.LabelDeploymentName:    deploymentName,
						bdv1.LabelInstanceGroupName: stS.Name,
					}))
					Expect(headlessService.Spec.Ports).To(Equal([]corev1.ServicePort{
						{
							Name:     "rep-server",
							Protocol: corev1.ProtocolTCP,
							Port:     1801,
						},
					}))
					Expect(headlessService.Spec.ClusterIP).To(Equal("None"))

					// Test affinity & tolerations
					Expect(stS.Spec.Affinity).To(BeNil())
					Expect(stS.Spec.Tolerations).To(Equal(tolerations))
				})
			})

			It("adds the canaryWatchTime of an instance group to an QuarksStatefulSet", func() {
				resources, err := act(bpmConfigs[1], m.InstanceGroups[1])
				Expect(err).ShouldNot(HaveOccurred())

				extStS := resources.InstanceGroups[0]
				Expect(extStS.Spec.Template.Annotations).To(HaveKeyWithValue(statefulset.AnnotationCanaryWatchTime, "1200000"))
			})

			It("combines the canaryWatchTime and custom annotations and adds them to QuarksStatefulSet", func() {
				m.InstanceGroups[1].Env.AgentEnvBoshConfig.Agent.Settings.Annotations = make(map[string]string)
				m.InstanceGroups[1].Env.AgentEnvBoshConfig.Agent.Settings.Annotations["custom-annotation"] = "bar"
				resources, err := act(bpmConfigs[1], m.InstanceGroups[1])
				Expect(err).ShouldNot(HaveOccurred())

				extStS := resources.InstanceGroups[0]
				Expect(extStS.Spec.Template.Annotations).To(HaveKeyWithValue(statefulset.AnnotationCanaryWatchTime, "1200000"))
				Expect(extStS.Spec.Template.Annotations).To(HaveKeyWithValue("custom-annotation", "bar"))
			})

			It("converts the AgentEnvBoshConfig information", func() {
				serviceAccount := "fake-service-account"
				automountServiceAccountToken := true
				m.InstanceGroups[1].Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName = serviceAccount
				m.InstanceGroups[1].Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken = &automountServiceAccountToken
				resources, err := act(bpmConfigs[1], m.InstanceGroups[1])
				Expect(err).ShouldNot(HaveOccurred())
				Expect(resources.InstanceGroups).To(HaveLen(1))

				// Test AgentEnvBoshConfig
				qJob := resources.InstanceGroups[0]
				Expect(qJob.Spec.Template.Spec.Template.Spec.ServiceAccountName).To(Equal(serviceAccount))
				Expect(*qJob.Spec.Template.Spec.Template.Spec.AutomountServiceAccountToken).To(Equal(automountServiceAccountToken))
			})
		})

		Context("when an active/passive probe is defined", func() {
			BeforeEach(func() {
				m, err = env.BOSHManifestWithActivePassiveProbes()
				Expect(err).NotTo(HaveOccurred())
			})

			It("passes it on to the QuarksStatefulSetSpec", func() {
				resources, err := act(bpm.Configs{
					"bpm1": {
						ActivePassiveProbes: map[string]corev1.Probe{
							"some-bpm-process": {
								PeriodSeconds: 2,
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"ls", "/"},
									},
								},
							},
							"another-bpm-process": {
								PeriodSeconds: 2,
								Handler: corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"find", "*"},
									},
								},
							},
						},
					},
				}, m.InstanceGroups[0])
				Expect(err).ShouldNot(HaveOccurred())

				qSts := resources.InstanceGroups[0]
				Expect(qSts.Spec.ActivePassiveProbes).ToNot(BeNil())
				Expect(qSts.Spec.ActivePassiveProbes["some-bpm-process"].Handler.Exec.Command).To(Equal([]string{"ls", "/"}))
				Expect(qSts.Spec.ActivePassiveProbes["another-bpm-process"].Handler.Exec.Command).To(Equal([]string{"find", "*"}))
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
				m, err = env.BOSHManifestWithMultiBPMProcesses()
				Expect(err).NotTo(HaveOccurred())

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

				resources, err = act(bpmConfigs[1], m.InstanceGroups[1])
				Expect(resources.InstanceGroups).To(HaveLen(1))
				Expect(err).ShouldNot(HaveOccurred())

				resources, err = act(bpmConfigs[2], m.InstanceGroups[2])
				Expect(err).ShouldNot(HaveOccurred())
				Expect(resources.InstanceGroups).To(HaveLen(1))
			})
		})

		Context("when the instance group name contains an underscore", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m, err = env.BOSHManifestCFRouting()
				Expect(err).NotTo(HaveOccurred())

				c, err := bpm.NewConfig([]byte(boshreleases.CFRouting))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"route_registrar": c},
				}

			})

			Context("when the lifecycle is set to errand", func() {
				It("converts the instance group to an QuarksJob", func() {
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.InstanceGroups).To(HaveLen(1))
				})
			})
		})

		Context("when the job contains a persistent disk declaration", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m, err = env.BOSHManifestWithBPMRelease()
				Expect(err).NotTo(HaveOccurred())

				c, err := bpm.NewConfig([]byte(boshreleases.EnablePersistentDiskBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"test-server": c},
				}

			})

			It("converts the disks and volume declarations when instance group has persistent disk declaration", func() {
				volumeFactory.GenerateBPMDisksReturns(manifest.Disks{
					{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name: "fake-pvc",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								StorageClassName: pointers.String("fake-storage-class"),
								AccessModes: []corev1.PersistentVolumeAccessMode{
									"ReadWriteOnce",
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("1G"),
									},
								},
							},
						},
					},
				}, nil)
				resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
				Expect(err).ShouldNot(HaveOccurred())

				// Test pvcs
				pvcs := resources.PersistentVolumeClaims
				Expect(pvcs[0]).NotTo(Equal(nil))
				Expect(pvcs[0].Name).To(Equal("fake-pvc"))
			})

			Context("when multiple BPM processes exist", func() {
				var (
					bpmConfigs []bpm.Configs
					bpmConfig1 bpm.Config
					bpmConfig2 bpm.Config
				)

				BeforeEach(func() {
					var err error
					m, err = env.BOSHManifestWithMultiBPMProcessesAndPersistentDisk()
					Expect(err).NotTo(HaveOccurred())

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
					containerFactory.JobsToContainersReturns([]corev1.Container{
						{
							Name: "fake-container",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "fake-volume-name",
									MountPath: "fake-mount-path",
								},
							},
						},
					}, nil)
					resources, err := act(bpmConfigs[0], m.InstanceGroups[0])
					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.Errands).To(HaveLen(1))
					containers := resources.Errands[0].Spec.Template.Spec.Template.Spec.Containers

					// Test shared volume setup
					Expect(containers[0].VolumeMounts[0].Name).To(Equal("fake-volume-name"))
					Expect(containers[0].VolumeMounts[0].MountPath).To(Equal("fake-mount-path"))
				})

				It("includes extra disk declarations from the instance group agent settings", func() {
					m, err = env.DefaultBOSHManifest()
					conf, err := bpm.NewConfig([]byte(boshreleases.DefaultBPMConfig))
					Expect(err).ShouldNot(HaveOccurred())

					bpmConfigs := []bpm.Configs{
						{"redis-server": conf},
						{"cflinuxfs3-rootfs-setup": conf},
					}

					c := bpmconverter.NewConverter(
						bpmconverter.NewVolumeFactory(),
						func(instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider manifest.ReleaseImageProvider, bpmConfigs bpm.Configs) bpmconverter.ContainerFactory {
							return bpmconverter.NewContainerFactory(
								instanceGroupName,
								"1",
								true,
								releaseImageProvider,
								bpmConfigs)
						})
					resources, err := c.Resources(*m, "foo", deploymentName, "1.2.3.4", "1", m.InstanceGroups[1], bpmConfigs[1], "1")

					Expect(err).ShouldNot(HaveOccurred())
					Expect(resources.InstanceGroups).To(HaveLen(1))

					volumes := resources.InstanceGroups[0].Spec.Template.Spec.Template.Spec.Volumes
					containers := resources.InstanceGroups[0].Spec.Template.Spec.Template.Spec.Containers

					// Test shared volume setup
					Expect(len(volumes)).To(Equal(7))
					Expect(volumes).To(ContainElement(
						corev1.Volume{
							Name:         "extravolume",
							VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
						},
					))
					Expect(containers[0].VolumeMounts).To(ContainElement(
						corev1.VolumeMount{Name: "extravolume", MountPath: "/var/vcap/data/rep"},
					))
				})
			})
		})

		Context("when affinity is provided", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m, err = env.BPMReleaseWithAffinity()
				Expect(err).NotTo(HaveOccurred())

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

		Context("when tolerations are provided", func() {
			var bpmConfigs []bpm.Configs

			BeforeEach(func() {
				m, err = env.BPMReleaseWithTolerations()
				Expect(err).NotTo(HaveOccurred())

				c, err := bpm.NewConfig([]byte(boshreleases.DefaultBPMConfig))
				Expect(err).ShouldNot(HaveOccurred())

				bpmConfigs = []bpm.Configs{
					{"test-server": c},
				}

			})

			It("adds tolerations into the spec", func() {
				r1, err := act(bpmConfigs[0], m.InstanceGroups[0])
				Expect(err).ShouldNot(HaveOccurred())

				ig1 := r1.InstanceGroups[0]
				Expect(ig1.Spec.Template.Spec.Template.Spec.Tolerations).To(Equal([]corev1.Toleration{
					{
						Key:      "key",
						Operator: "Equal",
						Value:    "value",
						Effect:   "NoSchedule",
					},
					{
						Key:      "key1",
						Operator: "Equal",
						Value:    "value1",
						Effect:   "NoExecute",
					},
				}))
			})
		})
	})

	Context("FilterLabels", func() {
		var labels map[string]string

		Context("map of labels", func() {

			BeforeEach(func() {
				labels = make(map[string]string)
				labels[bdv1.LabelDeploymentName] = "xxx"
				labels[bdv1.LabelDeploymentVersion] = "3"
			})

			It("deployment version is filtered", func() {
				filteredLabels := bpmconverter.FilterLabels(labels)
				Expect(filteredLabels).NotTo(HaveKey(bdv1.LabelDeploymentVersion))
			})
		})

	})
})
