package manifest_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	. "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	fakes "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/fakes"
)

var _ = Describe("ContainerFactory", func() {
	var (
		containerFactory     *ContainerFactory
		bpmConfigs           bpm.Configs
		releaseImageProvider *fakes.FakeReleaseImageProvider
		jobs                 []Job
	)

	BeforeEach(func() {
		releaseImageProvider = &fakes.FakeReleaseImageProvider{}
		releaseImageProvider.GetReleaseImageReturns("", nil)

		jobs = []Job{
			Job{Name: "fake-job"},
			Job{Name: "other-job"},
		}
	})

	JustBeforeEach(func() {
		containerFactory = NewContainerFactory("fake-manifest", "fake-ig", releaseImageProvider, bpmConfigs)
	})

	Context("JobsToContainers", func() {
		BeforeEach(func() {
			bpmConfigs = bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						bpm.Process{
							Name:          "fake-process",
							EphemeralDisk: true,
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
						},
					},
				},
				"other-job": bpm.Config{
					Processes: []bpm.Process{
						bpm.Process{
							Name:           "fake-process",
							Capabilities:   []string{"CHOWN", "AUDIT_CONTROL"},
							EphemeralDisk:  true,
							PersistentDisk: true,
						},
					},
				},
			}
		})

		act := func() ([]corev1.Container, error) {
			return containerFactory.JobsToContainers(jobs)
		}

		It("adds the sys volume", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers).To(HaveLen(2))
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
					Name:             "bpm-ephemeral-disk-fake-job",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s%s", VolumeEphemeralDirMountPath, "fake-job"),
					SubPath:          "",
					MountPropagation: nil,
				}))
			Expect(containers[1].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-ephemeral-disk-other-job",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s%s", VolumeEphemeralDirMountPath, "other-job"),
					SubPath:          "",
					MountPropagation: nil,
				}))
		})

		It("adds the persistent_disk volume", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers[0].VolumeMounts).ToNot(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-persistent-disk-fake-job",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s%s", VolumePersistentDirMountPath, "fake-job"),
					SubPath:          "",
					MountPropagation: nil,
				}))
			Expect(containers[1].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-persistent-disk-other-job",
					ReadOnly:         false,
					MountPath:        fmt.Sprintf("%s%s", VolumePersistentDirMountPath, "other-job"),
					SubPath:          "",
					MountPropagation: nil,
				}))

		})

		It("adds the additional volumes", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-additional-volume-fake-job-0",
					ReadOnly:         false,
					MountPath:        "/var/vcap/data/shared/foo",
					SubPath:          "",
					MountPropagation: nil,
				}))
			Expect(containers[0].VolumeMounts).To(ContainElement(
				corev1.VolumeMount{
					Name:             "bpm-additional-volume-fake-job-1",
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
			containerFactory = NewContainerFactory("fake-manifest", "fake-ig", releaseImageProvider, bpmConfigsWithError)
			actWithError := func() ([]corev1.Container, error) {
				return containerFactory.JobsToContainers(jobs)
			}
			_, err := actWithError()
			Expect(err).To(HaveOccurred())
		})

		It("adds linux capabilities to containers", func() {
			containers, err := act()
			Expect(err).ToNot(HaveOccurred())
			Expect(string(containers[1].SecurityContext.Capabilities.Add[0])).To(Equal("CHOWN"))
			Expect(string(containers[1].SecurityContext.Capabilities.Add[1])).To(Equal("AUDIT_CONTROL"))
		})
	})

	Context("JobsToInitContainers", func() {
		act := func(hasPersistentDisk bool) ([]corev1.Container, error) {
			return containerFactory.JobsToInitContainers(jobs, hasPersistentDisk)
		}

		Context("when multiple jobs are configured", func() {
			BeforeEach(func() {
				bpmConfigs = bpm.Configs{
					"fake-job":  bpm.Config{},
					"other-job": bpm.Config{},
				}
			})

			It("generates per job directories", func() {
				containers, err := act(false)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(5))
				Expect(containers[2].Name).To(Equal("create-dirs-fake-ig"))
				Expect(containers[2].Args).To(ContainElement("mkdir -p /var/vcap/data/fake-job /var/vcap/data/sys/log/fake-job /var/vcap/data/sys/run/fake-job /var/vcap/sys/log/fake-job /var/vcap/sys/run/fake-job /var/vcap/data/other-job /var/vcap/data/sys/log/other-job /var/vcap/data/sys/run/other-job /var/vcap/sys/log/other-job /var/vcap/sys/run/other-job"))
				Expect(containers[2].VolumeMounts).To(HaveLen(2))
			})

			It("generates one BOSH pre-start init container per job", func() {
				containers, err := act(false)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(5))
				Expect(containers[3].Name).To(Equal("bosh-pre-start-fake-job"))
				Expect(containers[3].Args).To(ContainElement(`if [ -x "/var/vcap/jobs/fake-job/bin/pre-start" ]; then "/var/vcap/jobs/fake-job/bin/pre-start"; fi`))
				Expect(containers[3].VolumeMounts).To(HaveLen(4))
				Expect(containers[4].Name).To(Equal("bosh-pre-start-other-job"))
				Expect(containers[4].Args).To(ContainElement(`if [ -x "/var/vcap/jobs/other-job/bin/pre-start" ]; then "/var/vcap/jobs/other-job/bin/pre-start"; fi`))
				Expect(containers[4].VolumeMounts).To(HaveLen(4))
			})

			It("generates BOSH pre-start init containers with persistent disk mounted on /var/vcap/store", func() {
				containers, err := act(true)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(5))
				Expect(containers[3].Name).To(Equal("bosh-pre-start-fake-job"))
				Expect(containers[3].Args).To(ContainElement(`if [ -x "/var/vcap/jobs/fake-job/bin/pre-start" ]; then "/var/vcap/jobs/fake-job/bin/pre-start"; fi`))
				Expect(containers[3].VolumeMounts).To(HaveLen(5))
				Expect(containers[3].VolumeMounts[4].Name).To(Equal("store-dir"))
				Expect(containers[3].VolumeMounts[4].MountPath).To(Equal("/var/vcap/store"))
				Expect(containers[4].Name).To(Equal("bosh-pre-start-other-job"))
				Expect(containers[4].Args).To(ContainElement(`if [ -x "/var/vcap/jobs/other-job/bin/pre-start" ]; then "/var/vcap/jobs/other-job/bin/pre-start"; fi`))
				Expect(containers[4].VolumeMounts).To(HaveLen(5))
				Expect(containers[4].VolumeMounts[4].Name).To(Equal("store-dir"))
				Expect(containers[4].VolumeMounts[4].MountPath).To(Equal("/var/vcap/store"))
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
							},
						},
					},
					"other-job": bpm.Config{},
				}
			})

			It("adds the pre start init container", func() {
				containers, err := act(false)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers).To(HaveLen(6))
				Expect(containers[5].Command).To(ContainElement("fake-cmd"))
			})

			It("generates hook init containers with bpm volumes for ephemeral disk", func() {
				containers, err := act(false)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers[5].VolumeMounts).To(HaveLen(8))
				Expect(containers[5].VolumeMounts).To(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-persistent-disk-fake-job",
						ReadOnly:         false,
						MountPath:        fmt.Sprintf("%s%s", VolumePersistentDirMountPath, "fake-job"),
						SubPath:          "",
						MountPropagation: nil,
					}))
			})

			It("generates hook init containers with bpm volumes for persistent disk", func() {
				containers, err := act(false)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers[5].VolumeMounts).To(HaveLen(8))
				Expect(containers[5].VolumeMounts).To(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-ephemeral-disk-fake-job",
						ReadOnly:         false,
						MountPath:        fmt.Sprintf("%s%s", VolumeEphemeralDirMountPath, "fake-job"),
						SubPath:          "",
						MountPropagation: nil,
					}))
			})

			It("generates hook init containers with bpm additional volumes", func() {
				containers, err := act(false)
				Expect(err).ToNot(HaveOccurred())
				Expect(containers[5].VolumeMounts).To(HaveLen(8))
				Expect(containers[5].VolumeMounts).To(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-additional-volume-fake-job-0",
						ReadOnly:         false,
						MountPath:        "/var/vcap/data/shared",
						SubPath:          "",
						MountPropagation: nil,
					}))
				Expect(containers[5].VolumeMounts).To(ContainElement(
					corev1.VolumeMount{
						Name:             "bpm-additional-volume-fake-job-1",
						ReadOnly:         true,
						MountPath:        "/var/vcap/store/foobar",
						SubPath:          "",
						MountPropagation: nil,
					}))
			})

		})
	})
})
