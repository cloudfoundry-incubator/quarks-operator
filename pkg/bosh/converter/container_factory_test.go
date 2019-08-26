package converter_test

import (
	"fmt"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	. "code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	fakes "code.cloudfoundry.org/cf-operator/pkg/bosh/converter/fakes"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

var _ = Describe("ContainerFactory", func() {
	var (
		containerFactory     *ContainerFactory
		bpmConfigs           bpm.Configs
		releaseImageProvider *fakes.FakeReleaseImageProvider
		jobs                 []bdm.Job
		defaultVolumeMounts  []corev1.VolumeMount
		bpmDisks             BPMResourceDisks
	)

	BeforeEach(func() {
		releaseImageProvider = &fakes.FakeReleaseImageProvider{}
		releaseImageProvider.GetReleaseImageReturns("", nil)

		jobs = []bdm.Job{
			bdm.Job{Name: "fake-job"},
			bdm.Job{Name: "other-job"},
		}

		defaultVolumeMounts = []corev1.VolumeMount{
			{
				Name:             VolumeSysDirName,
				ReadOnly:         false,
				MountPath:        VolumeSysDirMountPath,
				SubPath:          "",
				MountPropagation: nil,
			},
		}

		bpmDisks = BPMResourceDisks{
			{

				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName,
					MountPath: path.Join(VolumeDataDirMountPath, "fake-job"),
					SubPath:   "fake-job",
				},
				Labels: map[string]string{
					"job_name":  "fake-job",
					"ephemeral": "true",
				},
			},
			{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName,
					MountPath: path.Join(VolumeDataDirMountPath, "other-job"),
					SubPath:   "other-job",
				},
				Labels: map[string]string{
					"job_name":  "other-job",
					"ephemeral": "true",
				},
			},
			{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeStoreDirName,
					MountPath: path.Join(VolumeStoreDirMountPath, "fake-job"),
					SubPath:   "fake-job",
				},
				Labels: map[string]string{
					"job_name":   "fake-job",
					"persistent": "true",
				},
			},
			{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeStoreDirName,
					MountPath: path.Join(VolumeStoreDirMountPath, "other-job"),
					SubPath:   "other-job",
				},
				Labels: map[string]string{
					"job_name":   "other-job",
					"persistent": "true",
				},
			},
			{
				Volume: &corev1.Volume{
					Name:         "bpm-additional-volume-fake-job-fake-process-0",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:             "bpm-additional-volume-fake-job-fake-process-0",
					ReadOnly:         false,
					MountPath:        "/some/shared/foo",
					SubPath:          "",
					MountPropagation: nil,
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			},
			{
				Volume: &corev1.Volume{
					Name:         "bpm-additional-volume-fake-job-fake-process-1",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:             "bpm-additional-volume-fake-job-fake-process-1",
					ReadOnly:         false,
					MountPath:        "/var/vcap/store/foobar",
					SubPath:          "",
					MountPropagation: nil,
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			},
			{
				Volume: &corev1.Volume{
					Name:         "bpm-unrestricted-volume-fake-job-fake-process-0",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:             "bpm-unrestricted-volume-fake-job-fake-process-0",
					ReadOnly:         false,
					MountPath:        "/",
					SubPath:          "",
					MountPropagation: nil,
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			},
			{
				Volume: &corev1.Volume{
					Name:         "bpm-unrestricted-volume-fake-job-fake-process-1",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:             "bpm-unrestricted-volume-fake-job-fake-process-1",
					ReadOnly:         false,
					MountPath:        "/etc",
					SubPath:          "",
					MountPropagation: nil,
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			},
		}
	})

	JustBeforeEach(func() {
		containerFactory = NewContainerFactory("fake-manifest", "fake-ig", "v1", false, releaseImageProvider, bpmConfigs)
	})

	Context("JobsToContainers", func() {
		BeforeEach(func() {
			bpmConfigs = bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						bpm.Process{
							Name:           "fake-process",
							EphemeralDisk:  true,
							PersistentDisk: true,
							AdditionalVolumes: []bpm.Volume{
								{
									Path:     "/var/vcap/data/shared/foo",
									Writable: true,
								},
								{
									Path:     "/var/vcap/store/foobar",
									Writable: false,
								},
							},
							Unsafe: bpm.Unsafe{
								UnrestrictedVolumes: []bpm.Volume{
									{
										Path:     "/",
										Writable: true,
									},
									{
										Path:     "/etc",
										Writable: true,
									},
								},
							},
						},
					},
				},
				"other-job": bpm.Config{
					Processes: []bpm.Process{
						bpm.Process{
							Name:           "fake-process",
							Capabilities:   []string{"CHOWN", "AUDIT_CONTROL"},
							Env:            map[string]string{"a": "1", "b": "2"},
							EphemeralDisk:  true,
							PersistentDisk: true,
							Unsafe: bpm.Unsafe{
								UnrestrictedVolumes: []bpm.Volume{
									{
										Path:     "/etc/other-job/fake-process",
										Writable: false,
									},
								},
							},
						},
					},
				},
			}
		})

		act := func() ([]corev1.Container, error) {
			return containerFactory.JobsToContainers(jobs, defaultVolumeMounts, bpmDisks)
		}

		It("adds the default volume mounts passed", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).To(HaveLen(3))
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             VolumeSysDirName,
					ReadOnly:         false,
					MountPath:        VolumeSysDirMountPath,
					SubPath:          "",
					MountPropagation: nil,
				}))
		})

		It("adds the ephemeral_disk volume", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "data-dir",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s/%s", VolumeDataDirMountPath, "fake-job"),
					SubPath:          "fake-job",
					MountPropagation: nil,
				}))
			Expect(containers[1].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "data-dir",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s/%s", VolumeDataDirMountPath, "other-job"),
					SubPath:          "other-job",
					MountPropagation: nil,
				}))
		})

		It("adds the persistent_disk volume", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "store-dir",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s/%s", VolumeStoreDirMountPath, "fake-job"),
					SubPath:          "fake-job",
					MountPropagation: nil,
				}))
			Expect(containers[1].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "store-dir",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s/%s", VolumeStoreDirMountPath, "other-job"),
					SubPath:          "other-job",
					MountPropagation: nil,
				}))
		})

		It("adds the additional volumes", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-additional-volume-fake-job-fake-process-0",
					ReadOnly:         false,
					MountPath:        "/some/shared/foo",
					SubPath:          "",
					MountPropagation: nil,
				}))
			Expect(containers[0].VolumeMounts).ToNot(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-additional-volume-fake-job-fake-process-1",
					ReadOnly:         true,
					MountPath:        "/var/vcap/store/foobar",
					SubPath:          "",
					MountPropagation: nil,
				}))
		})

		It("adds the additional volumes but fail due to invalid path", func() {
			bpmConfigsWithError := bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						bpm.Process{
							Name: "foobar-process",
							AdditionalVolumes: []bpm.Volume{
								{
									Path: "/var/foo/bar",
								},
							},
						},
					},
				},
			}
			containerFactory = NewContainerFactory("fake-manifest", "fake-ig", "v1", false, releaseImageProvider, bpmConfigsWithError)
			actWithError := func() ([]corev1.Container, error) {
				return containerFactory.JobsToContainers(jobs, []corev1.VolumeMount{}, BPMResourceDisks{})
			}
			_, err := actWithError()
			Expect(err).To(HaveOccurred())
		})

		It("adds the unrestricted volumes", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-unrestricted-volume-fake-job-fake-process-0",
					ReadOnly:         false,
					MountPath:        "/",
					SubPath:          "",
					MountPropagation: nil,
				}))
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-unrestricted-volume-fake-job-fake-process-1",
					ReadOnly:         false,
					MountPath:        "/etc",
					SubPath:          "",
					MountPropagation: nil,
				}))
		})

		It("ensures the amount of volume mounts is correct", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(containers[0].VolumeMounts)).To(Equal(7))
		})

		It("adds linux capabilities to containers", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(containers[1].SecurityContext.Capabilities.Add[0])).To(Equal("CHOWN"))
			Expect(string(containers[1].SecurityContext.Capabilities.Add[1])).To(Equal("AUDIT_CONTROL"))
		})

		It("adds all environment variales to containers", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers[1].Env).To(HaveLen(2))
		})

		Context("with lifecycle events", func() {
			It("creates a preStop handler per job", func() {
				containers, err := act()
				Expect(err).ToNot(HaveOccurred())

				Expect(containers[0].Lifecycle).ToNot(BeNil())
				Expect(containers[0].Lifecycle.PreStop).ToNot(BeNil())
				Expect(containers[0].Lifecycle.PreStop.Exec.Command).To(ContainElement(ContainSubstring("/var/vcap/jobs/fake-job/bin/drain/")))

				Expect(containers[1].Lifecycle).ToNot(BeNil())
				Expect(containers[1].Lifecycle.PreStop).ToNot(BeNil())
				Expect(containers[1].Lifecycle.PreStop.Exec.Command).To(ContainElement(ContainSubstring("/var/vcap/jobs/other-job/bin/drain/")))
			})
		})

		Context("with logging sidecar container", func() {
			var (
				igJobs        []bdm.Job
				bpmJobConfigs bpm.Configs
			)
			BeforeEach(func() {
				igJobs = []bdm.Job{
					bdm.Job{
						Name: "foo",
					},
				}

				bpmJobConfigs = bpm.Configs{
					"foo": bpm.Config{
						Processes: []bpm.Process{
							bpm.Process{},
						},
					},
				}
			})
			It("enables it by default", func() {
				ig := bdm.InstanceGroup{
					Name: "fake-ig",
					Env: bdm.AgentEnv{
						AgentEnvBoshConfig: bdm.AgentEnvBoshConfig{
							Agent: bdm.Agent{},
						},
					},
					Jobs: igJobs,
				}

				disableSideCar := ig.Env.AgentEnvBoshConfig.Agent.Settings.DisableLogSidecar

				containerFactory := NewContainerFactory("fake-manifest", ig.Name, "v1", disableSideCar, releaseImageProvider, bpmJobConfigs)
				act := func() ([]corev1.Container, error) {
					return containerFactory.JobsToContainers(ig.Jobs, []corev1.VolumeMount{}, BPMResourceDisks{})
				}
				containers, err := act()

				Expect(err).ToNot(HaveOccurred())
				Expect(len(containers)).To(Equal(2))
			})

			It("disables it if specified", func() {
				ig := bdm.InstanceGroup{
					Name: "fake-ig",
					Env: bdm.AgentEnv{
						AgentEnvBoshConfig: bdm.AgentEnvBoshConfig{
							Agent: bdm.Agent{
								Settings: bdm.AgentSettings{
									DisableLogSidecar: true,
								},
							},
						},
					},
					Jobs: igJobs,
				}

				disableSideCar := ig.Env.AgentEnvBoshConfig.Agent.Settings.DisableLogSidecar

				containerFactory := NewContainerFactory("fake-manifest", ig.Name, "v1", disableSideCar, releaseImageProvider, bpmJobConfigs)
				act := func() ([]corev1.Container, error) {
					return containerFactory.JobsToContainers(ig.Jobs, []corev1.VolumeMount{}, BPMResourceDisks{})
				}
				containers, err := act()

				Expect(err).ToNot(HaveOccurred())
				Expect(len(containers)).To(Equal(1))
			})
		})
	})

	Context("JobsToInitContainers", func() {
		act := func() ([]corev1.Container, error) {
			return containerFactory.JobsToInitContainers(jobs, defaultVolumeMounts, bpmDisks)
		}

		Context("when multiple jobs are configured", func() {
			BeforeEach(func() {
				bpmConfigs = bpm.Configs{
					"fake-job":  bpm.Config{},
					"other-job": bpm.Config{},
				}
			})

			It("generates per job directories", func() {
				containers, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(6))
				Expect(containers[3].Name).To(Equal("create-dirs"))
				Expect(containers[3].Args).To(ContainElement("mkdir -p /var/vcap/data/fake-job /var/vcap/data/sys/log/fake-job /var/vcap/data/sys/run/fake-job /var/vcap/sys/log/fake-job /var/vcap/sys/run/fake-job /var/vcap/data/other-job /var/vcap/data/sys/log/other-job /var/vcap/data/sys/run/other-job /var/vcap/sys/log/other-job /var/vcap/sys/run/other-job"))
				Expect(containers[3].VolumeMounts).To(HaveLen(2))
			})

			It("generates one BOSH pre-start init container per job", func() {
				containers, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(6))
				Expect(containers[4].Name).To(Equal("bosh-pre-start-fake-job"))
				Expect(containers[4].Args).To(ContainElement(`if [ -x "/var/vcap/jobs/fake-job/bin/pre-start" ]; then "/var/vcap/jobs/fake-job/bin/pre-start"; fi`))
				Expect(containers[4].VolumeMounts).To(HaveLen(9))
				Expect(containers[5].Name).To(Equal("bosh-pre-start-other-job"))
				Expect(containers[5].Args).To(ContainElement(`if [ -x "/var/vcap/jobs/other-job/bin/pre-start" ]; then "/var/vcap/jobs/other-job/bin/pre-start"; fi`))
				Expect(containers[5].VolumeMounts).To(HaveLen(9))
			})
		})

		Context("when hooks are present", func() {
			BeforeEach(func() {
				bpmConfigs = bpm.Configs{
					"fake-job": bpm.Config{
						Processes: []bpm.Process{
							bpm.Process{Name: "fake-process",
								Hooks:          bpm.Hooks{PreStart: "fake-cmd"},
								EphemeralDisk:  true,
								PersistentDisk: true,
								AdditionalVolumes: []bpm.Volume{
									{
										Path:     "/var/vcap/data/shared",
										Writable: true,
									},
									{
										Path:     "/var/vcap/store/foobar",
										Writable: false,
									},
								},
								Unsafe: bpm.Unsafe{
									UnrestrictedVolumes: []bpm.Volume{
										{
											Path:     "/etc",
											Writable: false,
										},
										{
											Path:     "/var/vcap/store/foobar",
											Writable: false,
										},
									},
								},
							},
						},
					},
					"other-job": bpm.Config{},
				}
			})

			It("adds the pre start init container", func() {
				containers, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(7))
				Expect(containers[6].Args).To(ContainElement("fake-cmd"))
			})

			It("generates hook init containers with bpm volumes for ephemeral disk", func() {
				containers, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(containers[6].VolumeMounts).To(HaveLen(7))
				Expect(containers[6].VolumeMounts).To(ContainElement(
					corev1.VolumeMount{
						Name:             "data-dir",
						ReadOnly:         false,
						MountPath:        fmt.Sprintf("%s/%s", VolumeDataDirMountPath, "fake-job"),
						SubPath:          "fake-job",
						MountPropagation: nil,
					}))
			})

			It("generates hook init containers with bpm additional volumes", func() {
				containers, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(containers[6].VolumeMounts).To(HaveLen(7))
				Expect(containers[6].VolumeMounts).To(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-additional-volume-fake-job-fake-process-0",
						ReadOnly:         false,
						MountPath:        "/some/shared/foo",
						SubPath:          "",
						MountPropagation: nil,
					}))
				Expect(containers[6].VolumeMounts).ToNot(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-additional-volume-fake-job-fake-process-1",
						ReadOnly:         true,
						MountPath:        "/var/vcap/store/foobar",
						SubPath:          "",
						MountPropagation: nil,
					}))
			})

			It("generates hook init containers with bpm unrestricted volumes", func() {
				containers, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(containers[6].VolumeMounts).To(HaveLen(7))
				Expect(containers[6].VolumeMounts).To(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-unrestricted-volume-fake-job-fake-process-0",
						ReadOnly:         false,
						MountPath:        "/",
						SubPath:          "",
						MountPropagation: nil,
					}))
				Expect(containers[6].VolumeMounts).ToNot(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-unrestricted-volume-other-job-fake-process-0",
						ReadOnly:         true,
						MountPath:        "/etc/other-job/fake-process",
						SubPath:          "",
						MountPropagation: nil,
					}))
			})
		})
	})
})
