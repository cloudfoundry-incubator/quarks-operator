package bpmconverter_test

import (
	"fmt"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	. "code.cloudfoundry.org/cf-operator/pkg/bosh/bpmconverter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/disk"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("VolumeFactory", func() {
	var (
		factory       *VolumeFactoryImpl
		manifestName  string
		version       string
		namespace     string
		instanceGroup *bdm.InstanceGroup
		bpmConfigs    *bpm.Configs
	)

	BeforeEach(func() {
		manifestName = "fake-manifest-name"
		version = "1"
		namespace = "fake-namespace"
		instanceGroup = &bdm.InstanceGroup{
			Name: "fake-instance-group-name",
			Jobs: []bdm.Job{
				{
					Name: "fake-job",
				},
			},
		}
		bpmConfigs = &bpm.Configs{
			"fake-job": bpm.Config{},
		}
		factory = NewVolumeFactory()
	})

	Describe("GenerateDefaultDisks", func() {
		It("creates default disks", func() {
			disks := factory.GenerateDefaultDisks(manifestName, instanceGroup, version, namespace)

			Expect(disks).Should(HaveLen(5))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				Volume: &corev1.Volume{
					Name:         VolumeRenderingDataName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeRenderingDataName,
					MountPath: VolumeRenderingDataMountPath,
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				Volume: &corev1.Volume{
					Name:         VolumeJobsDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeJobsDirName,
					MountPath: VolumeJobsDirMountPath,
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				Volume: &corev1.Volume{
					Name:         VolumeDataDirName("fake-manifest-name", "fake-instance-group-name"),
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName("fake-manifest-name", "fake-instance-group-name"),
					MountPath: VolumeDataDirMountPath,
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				Volume: &corev1.Volume{
					Name:         VolumeSysDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeSysDirName,
					MountPath: VolumeSysDirMountPath,
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				Volume: &corev1.Volume{
					Name: "ig-resolved",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: fmt.Sprintf("%s.ig-resolved.%s-v%s", manifestName, instanceGroup.Name, version),
						},
					},
				},
			}))
		})
	})

	Describe("GenerateBPMDisks", func() {
		It("creates ephemeral disk", func() {
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							EphemeralDisk: true,
						},
					},
				},
			}

			disks, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(disks).Should(HaveLen(1))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName("fake-manifest-name", "fake-instance-group-name"),
					MountPath: path.Join(VolumeDataDirMountPath, "fake-job"),
					SubPath:   "fake-job",
				},
				Labels: map[string]string{
					"job_name":  "fake-job",
					"ephemeral": "true",
				},
			}))
		})

		It("uses a pvc for an ephemeral disk if so configured", func() {
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							EphemeralDisk: true,
						},
					},
				},
			}

			instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.EphemeralAsPVC = true

			disks, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(disks).Should(HaveLen(1))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				Volume: &corev1.Volume{
					Name: "fake-manifest-name-fake-instance-group-name-ephemeral",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "fake-manifest-name-fake-instance-group-name-ephemeral",
						},
					},
				},
				PersistentVolumeClaim: &corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-manifest-name-fake-instance-group-name-ephemeral",
						Namespace: "fake-namespace",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10240Mi"),
							},
						},
					},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName("fake-manifest-name", "fake-instance-group-name"),
					MountPath: path.Join(VolumeDataDirMountPath, "fake-job"),
					SubPath:   "fake-job",
				},
				Labels: map[string]string{
					"job_name":  "fake-job",
					"ephemeral": "true",
				},
			}))
		})

		It("creates persistent disk", func() {
			instanceGroup.PersistentDisk = pointers.Int(1)
			instanceGroup.PersistentDiskType = "fake-storage-class"
			persistentVolumeClaim := corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.Sanitize(fmt.Sprintf("%s-%s-%s", manifestName, instanceGroup.Name, "pvc")),
					Namespace: namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(fmt.Sprintf("%d%s", *instanceGroup.PersistentDisk, "Mi")),
						},
					},
					StorageClassName: pointers.String("fake-storage-class"),
				},
			}
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							PersistentDisk: true,
						},
					},
				},
			}

			disks, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(disks).Should(HaveLen(1))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				PersistentVolumeClaim: &persistentVolumeClaim,
				Volume: &corev1.Volume{
					Name: "fake-manifest-name-fake-instance-group-name-pvc",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: persistentVolumeClaim.Name,
						},
					},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      "fake-manifest-name-fake-instance-group-name-pvc",
					MountPath: path.Join(VolumeStoreDirMountPath, "fake-job"),
					SubPath:   "fake-job",
				},
				Labels: map[string]string{
					"job_name":   "fake-job",
					"persistent": "true",
				},
			}))
		})

		It("creates additional volumes", func() {
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							Name: "fake-process",
							AdditionalVolumes: []bpm.Volume{
								{
									Path: "/var/vcap/data/add1",
								},
								{
									Path: "/var/vcap/store/add2",
								},
								{
									Path: "/var/vcap/sys/run/add3",
								},
							},
						},
					},
				},
			}

			instanceGroup.PersistentDisk = pointers.Int(42)
			disks, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(disks).Should(HaveLen(3))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName("fake-manifest-name", "fake-instance-group-name"),
					ReadOnly:  true,
					MountPath: "/var/vcap/data/add1",
					SubPath:   "add1",
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      "fake-manifest-name-fake-instance-group-name-pvc",
					ReadOnly:  true,
					MountPath: "/var/vcap/store/add2",
					SubPath:   "add2",
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeSysDirName,
					ReadOnly:  true,
					MountPath: "/var/vcap/sys/run/add3",
					SubPath:   "run/add3",
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			}))
		})

		It("creates unrestricted volumes", func() {
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							Name: "fake-process",
							Unsafe: bpm.Unsafe{
								UnrestrictedVolumes: []bpm.Volume{
									{
										Path: "/var/vcap/data/add1",
									},
									{
										Path: "/var/vcap/store/add2",
									},
									{
										Path: "/var/vcap/sys/run/add3",
									},
									{
										Path: "/unrestricted/fake-path",
									},
								},
							},
						},
					},
				},
			}

			instanceGroup.PersistentDisk = pointers.Int(42)

			disks, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(disks).Should(HaveLen(4))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName("fake-manifest-name", "fake-instance-group-name"),
					ReadOnly:  true,
					MountPath: "/var/vcap/data/add1",
					SubPath:   "add1",
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      "fake-manifest-name-fake-instance-group-name-pvc",
					ReadOnly:  true,
					MountPath: "/var/vcap/store/add2",
					SubPath:   "add2",
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeSysDirName,
					ReadOnly:  true,
					MountPath: "/var/vcap/sys/run/add3",
					SubPath:   "run/add3",
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			}))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				Volume: &corev1.Volume{
					Name:         "bpm-unrestricted-volume-fake-job-fake-process-0",
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      "bpm-unrestricted-volume-fake-job-fake-process-0",
					ReadOnly:  true,
					MountPath: "/unrestricted/fake-path",
				},
				Labels: map[string]string{
					"job_name":     "fake-job",
					"process_name": "fake-process",
				},
			}))
		})

		It("skips unrestricted job volume already mounted", func() {
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							Name: "fake-process",
							Unsafe: bpm.Unsafe{
								UnrestrictedVolumes: []bpm.Volume{
									{
										Path: "/var/vcap/jobs/fake-unrestricted",
									},
								},
							},
						},
					},
				},
			}

			disks, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(disks).Should(HaveLen(0))
		})

		It("handles error when persistent disk wasn't declared", func() {
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							PersistentDisk: true,
						},
					},
				},
			}

			_, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring("instance group 'fake-instance-group-name' doesn't have any persistent disk declaration"))
		})

		It("handles error when additional volume wasn't a path inside '/var/vcap/'", func() {
			bpmConfigs = &bpm.Configs{
				"fake-job": bpm.Config{
					Processes: []bpm.Process{
						{
							Name: "fake-process",
							AdditionalVolumes: []bpm.Volume{
								{
									Path: "/sys/add1",
								},
							},
						},
					},
				},
			}

			_, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).Should(HaveOccurred())
			Expect(err.Error()).Should(ContainSubstring(fmt.Sprintf("the '%s' path, must be a path inside"+
				" '/var/vcap/data', '/var/vcap/store' or '/var/vcap/sys/run', for a path outside these,"+
				" you must use the unrestricted_volumes key", "/sys/add1")))
		})
	})
})
