package factory_test

import (
	"fmt"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/disk"
	. "code.cloudfoundry.org/cf-operator/pkg/bosh/factory"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("VolumeFactory", func() {
	var (
		factory       *VolumeFactory
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
			disks := factory.GenerateDefaultDisks(manifestName, instanceGroup.Name, version, namespace)

			Expect(disks).Should(HaveLen(6))
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
					Name:         VolumeDataDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName,
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
					Name: fmt.Sprintf("desired-manifest-v%s", version),
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: fmt.Sprintf("%s.desired-manifest-v%s", manifestName, version),
						},
					},
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
			Expect(err).NotTo(HaveOccurred())

			Expect(disks).Should(HaveLen(1))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName,
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
			instanceGroup.PersistentDisk = util.Int(1)
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
					StorageClassName: util.String("fake-storage-class"),
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
			Expect(err).NotTo(HaveOccurred())

			Expect(disks).Should(HaveLen(1))
			Expect(disks).Should(ContainElement(disk.BPMResourceDisk{
				PersistentVolumeClaim: &persistentVolumeClaim,
				Volume: &corev1.Volume{
					Name: VolumeStoreDirName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: persistentVolumeClaim.Name,
						},
					},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeStoreDirName,
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
							AdditionalVolumes: []bpm.Volume{
								{
									Path: "/var/vcap/data/add1",
								},
								{
									Path: "/var/vcap/data/add2",
								},
								{
									Path: "/var/vcap/sys/add3",
								},
							},
						},
					},
				},
			}

			disks, err := factory.GenerateBPMDisks(manifestName, instanceGroup, *bpmConfigs, namespace)
			Expect(err).NotTo(HaveOccurred())

			Expect(disks).Should(HaveLen(1))
		})
	})
})
