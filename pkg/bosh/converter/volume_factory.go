package converter

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

const (
	secretsPath         = "/var/run/secrets/variables/"
	withOpsManifestPath = "/var/run/secrets/deployment/"
	// releaseSourceName is the folder for release sources
	releaseSourceName        = "instance-group"
	resolvedPropertiesFormat = "/var/run/secrets/resolved-properties/%s"
)

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
