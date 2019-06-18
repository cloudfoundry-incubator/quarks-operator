package manifest

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// VolumeRenderingDataName is the volume name for the rendering data.
	VolumeRenderingDataName = "rendering-data"
	// VolumeRenderingDataMountPath is the mount path for the rendering data.
	VolumeRenderingDataMountPath = "/var/vcap/all-releases"

	// VolumeJobsDirName is the volume name for the jobs directory.
	VolumeJobsDirName = "jobs-dir"
	// VolumeJobsDirMountPath is the mount path for the jobs directory.
	VolumeJobsDirMountPath = "/var/vcap/jobs"

	// VolumeJobsSrcDirName is the volume name for the jobs-src directory.
	VolumeJobsSrcDirName = "jobs-src-dir"
	// VolumeJobsSrcDirMountPath is the mount path for the jobs-src directory.
	VolumeJobsSrcDirMountPath = "/var/vcap/jobs-src"

	// VolumeDataDirName is the volume name for the data directory.
	VolumeDataDirName = "data-dir"
	// VolumeDataDirMountPath is the mount path for the data directory.
	VolumeDataDirMountPath = "/var/vcap/data"

	// VolumeSysDirName is the volume name for the sys directory.
	VolumeSysDirName = "sys-dir"
	// VolumeSysDirMountPath is the mount path for the sys directory.
	VolumeSysDirMountPath = "/var/vcap/sys"

	// VolumeStoreDirName is the volume name for the store directory.
	VolumeStoreDirName = "store-dir"
	// VolumeStoreDirMountPath is the mount path for the store directory.
	VolumeStoreDirMountPath = "/var/vcap/store"

	// VolumeEphemeralDirName is the volume name for the ephemeral disk directory.
	VolumeEphemeralDirName = "bpm-ephemeral-disk"
	// VolumeEphemeralDirMountPath is the mount path for the ephemeral directory.
	VolumeEphemeralDirMountPath = "/var/vcap/data"

	// AdditionalVolumeBaseName helps in building an additional volume name together with
	// the index under the additional_volumes bpm list inside the bpm process schema.
	AdditionalVolumeBaseName = "bpm-additional-volume"

	// AdditionalVolumesRegex ensures only a valid path is defined
	// under the additional_volumes bpm list inside the bpm process schema.
	AdditionalVolumesRegex = "((/var/vcap/data/.+)|(/var/vcap/store/.+)|(/var/vcap/sys/run/.+))"

	// AdditionalVolumesVcapStoreRegex ensures that the path is of the form
	// /var/vcap/store.
	AdditionalVolumesVcapStoreRegex = "(/var/vcap/store/.+)"

	// UnrestrictedVolumeBaseName is the volume name for the unrestricted ones.
	UnrestrictedVolumeBaseName = "bpm-unrestricted-volume"
)

func generateDefaultDisks(manifestName string, instanceGroup *InstanceGroup, namespace string) BPMResourceDisks {
	interpolatedManifestSecretName := names.CalculateIGSecretName(
		names.DeploymentSecretTypeManifestAndVars,
		manifestName,
		VarInterpolationContainerName,
		true,
	)
	resolvedPropertiesSecretName := names.CalculateIGSecretName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		manifestName,
		instanceGroup.Name,
		true,
	)

	defaultDisks := BPMResourceDisks{
		{
			Volume: &corev1.Volume{
				Name:         VolumeRenderingDataName,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
			VolumeMount: &corev1.VolumeMount{
				Name:      VolumeRenderingDataName,
				MountPath: VolumeRenderingDataMountPath,
			},
		},
		{
			Volume: &corev1.Volume{
				Name:         VolumeJobsDirName,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
			VolumeMount: &corev1.VolumeMount{
				Name:      VolumeJobsDirName,
				MountPath: VolumeJobsDirMountPath,
			},
		},
		{
			// For ephemeral job data.
			// https://bosh.io/docs/vm-config/#jobs-and-packages
			Volume: &corev1.Volume{
				Name:         VolumeDataDirName,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
			VolumeMount: &corev1.VolumeMount{
				Name:      VolumeDataDirName,
				MountPath: VolumeDataDirMountPath,
			},
		},
		{
			Volume: &corev1.Volume{
				Name:         VolumeSysDirName,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			},
			VolumeMount: &corev1.VolumeMount{
				Name:      VolumeSysDirName,
				MountPath: VolumeSysDirMountPath,
			},
		},
		{
			Volume: &corev1.Volume{
				Name: generateVolumeName(interpolatedManifestSecretName),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: interpolatedManifestSecretName,
					},
				},
			},
		},
		{
			Volume: &corev1.Volume{
				Name: generateVolumeName(resolvedPropertiesSecretName),
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: resolvedPropertiesSecretName,
					},
				},
			},
		},
	}

	// Create a persistent volume claim if specified in spec.
	if instanceGroup.PersistentDisk != nil && *instanceGroup.PersistentDisk > 0 {
		persistentVolumeClaim := generatePersistentVolumeClaim(manifestName, instanceGroup, namespace)
		persistentDisk := BPMResourceDisk{
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
				MountPath: VolumeStoreDirMountPath,
			},
		}
		defaultDisks = append(defaultDisks, persistentDisk)
	}

	return defaultDisks
}

// generateBPMDisks defines any other volumes required to be mounted,
// based on the bpm process schema definition. This looks for:
// - ephemeral_disk (boolean)
// - persistent_disk (boolean)
// - additional_volumes (list of volumes)
// - unrestricted_volumes (list of volumes)
func generateBPMDisks(manifestName string, instanceGroup *InstanceGroup, bpmConfigs bpm.Configs) (BPMResourceDisks, error) {
	bpmDisks := make(BPMResourceDisks, 0)

	rAdditionalVolumes := regexp.MustCompile(AdditionalVolumesRegex)

	for _, job := range instanceGroup.Jobs {
		bpmConfig := bpmConfigs[job.Name]
		hasEphemeralDisk := false
		for _, process := range bpmConfig.Processes {
			if !hasEphemeralDisk && process.EphemeralDisk {
				hasEphemeralDisk = true
			}

			// TODO: skip this, while we need to figure it out a better way
			// to define persistenVolumeClaims for jobs
			// if process.PersistentDisk {
			// }

			for i, additionalVolume := range process.AdditionalVolumes {
				match := rAdditionalVolumes.MatchString(additionalVolume.Path)
				if !match {
					return nil, fmt.Errorf("The %s path, must be a path inside"+
						" /var/vcap/data, /var/vcap/store or /var/vcap/sys/run, for a path outside these,"+
						" you must use the unrestricted_volumes key", additionalVolume.Path)
				}
				volumeName := fmt.Sprintf("%s-%s-%s-%b", AdditionalVolumeBaseName, job.Name, process.Name, i)
				additionalDisk := BPMResourceDisk{
					Volume: &corev1.Volume{
						Name:         volumeName,
						VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
					},
					// TODO: How to map the following bpm volume schema fields:
					// - allow_executions
					// - mount_only
					// into a corev1.VolumeMount.
					VolumeMount: &corev1.VolumeMount{
						Name:      volumeName,
						ReadOnly:  !additionalVolume.Writable,
						MountPath: additionalVolume.Path,
					},
					Labels: map[string]string{
						"job_name":     job.Name,
						"process_name": process.Name,
					},
				}
				bpmDisks = append(bpmDisks, additionalDisk)
			}

			for i, unrestrictedVolume := range process.Unsafe.UnrestrictedVolumes {
				match := rAdditionalVolumes.MatchString(unrestrictedVolume.Path)
				if match {
					return nil, fmt.Errorf("The %s path, must be a path outside"+
						" /var/vcap/data, /var/vcap/store or /var/vcap/sys/run, for a path inside these,"+
						" you must use the additional_volumes key", unrestrictedVolume.Path)
				}
				volumeName := fmt.Sprintf("%s-%s-%s-%b", UnrestrictedVolumeBaseName, job.Name, process.Name, i)
				unrestrictedDisk := BPMResourceDisk{
					Volume: &corev1.Volume{
						Name:         volumeName,
						VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
					},
					VolumeMount: &corev1.VolumeMount{
						Name:      volumeName,
						ReadOnly:  !unrestrictedVolume.Writable,
						MountPath: unrestrictedVolume.Path,
					},
					Labels: map[string]string{
						"job_name":     job.Name,
						"process_name": process.Name,
					},
				}
				bpmDisks = append(bpmDisks, unrestrictedDisk)
			}
		}

		if hasEphemeralDisk {
			ephemeralDiskName := fmt.Sprintf("%s-%s", VolumeEphemeralDirName, job.Name)
			ephemeralDisk := BPMResourceDisk{
				Volume: &corev1.Volume{
					Name:         ephemeralDiskName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
				VolumeMount: &corev1.VolumeMount{
					Name:      ephemeralDiskName,
					MountPath: path.Join(VolumeEphemeralDirMountPath, job.Name),
				},
				Labels: map[string]string{
					"job_name":  job.Name,
					"ephemeral": "true",
				},
			}
			bpmDisks = append(bpmDisks, ephemeralDisk)
		}
	}
	return bpmDisks, nil
}

// generateVolumeName generate volume name based on secret name
func generateVolumeName(secretName string) string {
	nameSlices := strings.Split(secretName, ".")
	volName := ""
	if len(nameSlices) > 1 {
		volName = nameSlices[1]
	} else {
		volName = nameSlices[0]
	}
	return volName
}

func generatePersistentVolumeClaim(manifestName string, instanceGroup *InstanceGroup, namespace string) corev1.PersistentVolumeClaim {
	// Spec of a persistent volumeclaim.
	persistentVolumeClaim := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", manifestName, instanceGroup.Name, "pvc"),
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(fmt.Sprintf("%d%s", *instanceGroup.PersistentDisk, "Mi")),
				},
			},
		},
	}

	// add storage class if specified
	if instanceGroup.PersistentDiskType != "" {
		persistentVolumeClaim.Spec.StorageClassName = &instanceGroup.PersistentDiskType
	}

	return persistentVolumeClaim
}
