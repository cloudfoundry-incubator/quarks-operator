package converter

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/disk"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
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
	// VolumeDataDirMountPath is the mount path for the ephemeral (data) directory.
	VolumeDataDirMountPath = bdm.DataDir

	// VolumeSysDirName is the volume name for the sys directory.
	VolumeSysDirName = "sys-dir"
	// VolumeSysDirMountPath is the mount path for the sys directory.
	VolumeSysDirMountPath = bdm.SysDir

	// VolumeStoreDirName is the volume name for the store directory.
	VolumeStoreDirName = "store-dir"
	// VolumeStoreDirMountPath is the mount path for the store directory.
	VolumeStoreDirMountPath = "/var/vcap/store"

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

	secretsPath         = "/var/run/secrets/variables/"
	withOpsManifestPath = "/var/run/secrets/deployment/"
	// releaseSourceName is the folder for release sources
	releaseSourceName        = "instance-group"
	resolvedPropertiesFormat = "/var/run/secrets/resolved-properties/%s"
)

// VolumeFactoryImpl is a concrete implementation of VolumeFactoryImpl
type VolumeFactoryImpl struct {
}

// NewVolumeFactory returns a concrete implementation of VolumeFactory
func NewVolumeFactory() *VolumeFactoryImpl {
	return &VolumeFactoryImpl{}
}

// GenerateDefaultDisks defines default disks. This looks for:
// - the rendering data volume
// - the the jobs volume
// - the ephemeral (data) volume
// - the sys volume
// - the "not interpolated" manifest volume
// - resolved properties data volume
func (f *VolumeFactoryImpl) GenerateDefaultDisks(manifestName string, instanceGroupName string, version string, namespace string) disk.BPMResourceDisks {
	desiredManifestName := names.DesiredManifestName(manifestName, version)
	resolvedPropertiesSecretName := names.CalculateIGSecretName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		manifestName,
		instanceGroupName,
		version,
	)

	defaultDisks := disk.BPMResourceDisks{
		{
			Volume:      renderingVolume(),
			VolumeMount: renderingVolumeMount(),
		},
		{
			Volume:      jobsDirVolume(),
			VolumeMount: jobsDirVolumeMount(),
		},
		{
			// For ephemeral job data.
			// https://bosh.io/docs/vm-config/#jobs-and-packages
			Volume:      dataDirVolume(),
			VolumeMount: dataDirVolumeMount(),
		},
		{
			Volume:      sysDirVolume(),
			VolumeMount: sysDirVolumeMount(),
		},
		{
			Volume: withOpsVolume(desiredManifestName),
		},
		{
			Volume: resolvedPropertiesVolume(resolvedPropertiesSecretName),
		},
	}

	return defaultDisks
}

// GenerateBPMDisks defines any other volumes required to be mounted,
// based on the bpm process schema definition. This looks for:
// - ephemeral_disk (boolean)
// - persistent_disk (boolean)
// - additional_volumes (list of volumes)
// - unrestricted_volumes (list of volumes)
func (f *VolumeFactoryImpl) GenerateBPMDisks(manifestName string, instanceGroup *bdm.InstanceGroup, bpmConfigs bpm.Configs, namespace string) (disk.BPMResourceDisks, error) {
	bpmDisks := make(disk.BPMResourceDisks, 0)

	rAdditionalVolumes := regexp.MustCompile(AdditionalVolumesRegex)

	for _, job := range instanceGroup.Jobs {
		bpmConfig := bpmConfigs[job.Name]
		hasEphemeralDisk := false
		hasPersistentDisk := false
		for _, process := range bpmConfig.Processes {
			if !hasEphemeralDisk && process.EphemeralDisk {
				hasEphemeralDisk = true
			}
			if !hasPersistentDisk && process.PersistentDisk {
				hasPersistentDisk = true
			}

			// Because we use subpaths for data, store or sys mounts, we want to
			// treat unrestricted volumes as additional volumes
			// /var/vcap/jobs is already mounted everywhere for quarks, so we ignore anything in there
			filteredUnrestrictedVolumes := []bpm.Volume{}
			for _, unrestrictedVolume := range process.Unsafe.UnrestrictedVolumes {
				if strings.HasPrefix(unrestrictedVolume.Path, VolumeJobsDirMountPath) {
					continue
				}

				if strings.HasPrefix(unrestrictedVolume.Path, VolumeDataDirMountPath) ||
					strings.HasPrefix(unrestrictedVolume.Path, VolumeStoreDirMountPath) ||
					strings.HasPrefix(unrestrictedVolume.Path, VolumeSysDirMountPath) {
					// Add it to additional volumes
					process.AdditionalVolumes = append(process.AdditionalVolumes, unrestrictedVolume)
					continue
				}

				filteredUnrestrictedVolumes = append(filteredUnrestrictedVolumes, unrestrictedVolume)
			}
			process.Unsafe.UnrestrictedVolumes = filteredUnrestrictedVolumes

			for _, additionalVolume := range process.AdditionalVolumes {
				match := rAdditionalVolumes.MatchString(additionalVolume.Path)
				if !match {
					return nil, errors.Errorf("the '%s' path, must be a path inside"+
						" '/var/vcap/data', '/var/vcap/store' or '/var/vcap/sys/run', for a path outside these,"+
						" you must use the unrestricted_volumes key", additionalVolume.Path)
				}

				var (
					err        error
					volumeName string
					subPath    string
				)
				if strings.HasPrefix(additionalVolume.Path, VolumeDataDirMountPath) {
					volumeName = VolumeDataDirName
					subPath, err = filepath.Rel(VolumeDataDirMountPath, additionalVolume.Path)
				}
				if strings.HasPrefix(additionalVolume.Path, VolumeStoreDirMountPath) {
					volumeName = VolumeStoreDirName
					subPath, err = filepath.Rel(VolumeStoreDirMountPath, additionalVolume.Path)
				}
				if strings.HasPrefix(additionalVolume.Path, VolumeSysDirMountPath) {
					volumeName = VolumeSysDirName
					subPath, err = filepath.Rel(VolumeSysDirMountPath, additionalVolume.Path)
				}

				if err != nil {
					return nil, errors.Wrapf(err, "failed to calculate subpath for additional volume mount '%s'", additionalVolume.Path)
				}

				additionalDisk := disk.BPMResourceDisk{
					VolumeMount: &corev1.VolumeMount{
						Name:      volumeName,
						ReadOnly:  !additionalVolume.Writable,
						MountPath: additionalVolume.Path,
						SubPath:   subPath,
					},
					Labels: map[string]string{
						"job_name":     job.Name,
						"process_name": process.Name,
					},
				}
				bpmDisks = append(bpmDisks, additionalDisk)
			}

			for i, unrestrictedVolume := range process.Unsafe.UnrestrictedVolumes {
				volumeName := names.Sanitize(fmt.Sprintf("%s-%s-%s-%b", UnrestrictedVolumeBaseName, job.Name, process.Name, i))
				unrestrictedDisk := disk.BPMResourceDisk{
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
			ephemeralDisk := disk.BPMResourceDisk{
				VolumeMount: &corev1.VolumeMount{
					Name:      VolumeDataDirName,
					MountPath: path.Join(VolumeDataDirMountPath, job.Name),
					SubPath:   job.Name,
				},
				Labels: map[string]string{
					"job_name":  job.Name,
					"ephemeral": "true",
				},
			}
			bpmDisks = append(bpmDisks, ephemeralDisk)
		}

		if hasPersistentDisk {
			if instanceGroup.PersistentDisk == nil || *instanceGroup.PersistentDisk <= 0 {
				return bpmDisks, errors.Errorf("job '%s' wants to use persistent disk"+
					" but instance group '%s' doesn't have any persistent disk declaration", job.Name, instanceGroup.Name)
			}

			persistentVolumeClaim := generatePersistentVolumeClaim(manifestName, instanceGroup, namespace)

			// Specify the job sub-path inside of the instance group PV
			bpmPersistentDisk := disk.BPMResourceDisk{
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
					MountPath: path.Join(VolumeStoreDirMountPath, job.Name),
					SubPath:   job.Name,
				},
				Labels: map[string]string{
					"job_name":   job.Name,
					"persistent": "true",
				},
			}
			bpmDisks = append(bpmDisks, bpmPersistentDisk)
		}
	}

	return bpmDisks, nil
}

func generatePersistentVolumeClaim(manifestName string, instanceGroup *bdm.InstanceGroup, namespace string) corev1.PersistentVolumeClaim {
	// Spec of a persistentVolumeClaim
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
		},
	}

	// add storage class if specified
	if instanceGroup.PersistentDiskType != "" {
		persistentVolumeClaim.Spec.StorageClassName = &instanceGroup.PersistentDiskType
	}

	return persistentVolumeClaim
}

// withOpsVolume is a volume for the "not interpolated" manifest,
// that has the ops files applied, but still contains '((vars))'
func withOpsVolume(name string) *corev1.Volume {
	return &corev1.Volume{
		Name: generateVolumeName(name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	}
}

// withOpsVolumeMount mount for the with-ops manifest
func withOpsVolumeMount(name string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      generateVolumeName(name),
		MountPath: withOpsManifestPath,
		ReadOnly:  true,
	}
}

// variableVolume gives the volume definition for the variables content
func variableVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: generateVolumeName(name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	}
}

// variableVolumeMount is the volume mount to file 'varName' for a variables content
func variableVolumeMount(name string, varName string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      generateVolumeName(name),
		MountPath: secretsPath + varName,
		ReadOnly:  true,
	}
}

// noVarsVolume returns an EmptyVolume
func noVarsVolume() corev1.Volume {
	return corev1.Volume{
		Name: generateVolumeName("no-vars"),
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// noVarsVolumeMount returns the corresponding VolumeMount
func noVarsVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      generateVolumeName("no-vars"),
		MountPath: secretsPath,
		ReadOnly:  true,
	}
}

func releaseSourceVolume() corev1.Volume {
	return corev1.Volume{
		Name: generateVolumeName(releaseSourceName),
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func releaseSourceVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      generateVolumeName(releaseSourceName),
		MountPath: VolumeRenderingDataMountPath,
	}
}

func renderingVolume() *corev1.Volume {
	return &corev1.Volume{
		Name:         VolumeRenderingDataName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}

func renderingVolumeMount() *corev1.VolumeMount {
	return &corev1.VolumeMount{
		Name:      VolumeRenderingDataName,
		MountPath: VolumeRenderingDataMountPath,
	}
}

func jobsDirVolume() *corev1.Volume {
	return &corev1.Volume{
		Name:         VolumeJobsDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}

func jobsDirVolumeMount() *corev1.VolumeMount {
	return &corev1.VolumeMount{
		Name:      VolumeJobsDirName,
		MountPath: VolumeJobsDirMountPath,
	}
}

func resolvedPropertiesVolume(name string) *corev1.Volume {
	return &corev1.Volume{
		Name: generateVolumeName(name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	}
}

func resolvedPropertiesVolumeMount(name string, instanceGroupName string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      generateVolumeName(name),
		MountPath: fmt.Sprintf(resolvedPropertiesFormat, instanceGroupName),
		ReadOnly:  true,
	}
}

// For ephemeral job data.
// https://bosh.io/docs/vm-config/#jobs-and-packages
func dataDirVolume() *corev1.Volume {
	return &corev1.Volume{
		Name:         VolumeDataDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}

func dataDirVolumeMount() *corev1.VolumeMount {
	return &corev1.VolumeMount{
		Name:      VolumeDataDirName,
		MountPath: VolumeDataDirMountPath,
	}
}

func sysDirVolume() *corev1.Volume {
	return &corev1.Volume{
		Name:         VolumeSysDirName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
}

func sysDirVolumeMount() *corev1.VolumeMount {
	return &corev1.VolumeMount{
		Name:      VolumeSysDirName,
		MountPath: VolumeSysDirMountPath,
	}
}

func deduplicateVolumeMounts(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	result := []corev1.VolumeMount{}
	uniqueMounts := map[string]struct{}{}

	for _, volumeMount := range volumeMounts {
		if _, ok := uniqueMounts[volumeMount.MountPath]; ok {
			continue
		}

		uniqueMounts[volumeMount.MountPath] = struct{}{}
		result = append(result, volumeMount)
	}

	return result
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
	return names.Sanitize(volName)
}
