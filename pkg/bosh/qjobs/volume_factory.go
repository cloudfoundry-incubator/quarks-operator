package qjobs

import (
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpmconverter"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

const (
	secretsPath  = "/var/run/secrets/variables/"
	manifestPath = "/var/run/secrets/deployment/"

	// releaseSourceName is the folder for release sources
	releaseSourceName = "instance-group"
)

// withOpsVolume is a volume for the "not interpolated" manifest,
// that has the ops files applied, but still contains '((vars))'
func withOpsVolume(name string) *corev1.Volume {
	return &corev1.Volume{
		Name: names.VolumeName(name),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	}
}

// manifestVolumeMount mount for the manifest
func manifestVolumeMount(name string) corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      names.VolumeName(name),
		MountPath: manifestPath,
		ReadOnly:  true,
	}
}

// variableVolume gives the volume definition for the variables content
func variableVolume(name string) corev1.Volume {
	return corev1.Volume{
		Name: names.VolumeName(name),
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
		Name:      names.VolumeName(name),
		MountPath: secretsPath + varName,
		ReadOnly:  true,
	}
}

// noVarsVolume returns an EmptyVolume
func noVarsVolume() corev1.Volume {
	return corev1.Volume{
		Name: names.VolumeName("no-vars"),
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

// noVarsVolumeMount returns the corresponding VolumeMount
func noVarsVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      names.VolumeName("no-vars"),
		MountPath: secretsPath,
		ReadOnly:  true,
	}
}

func releaseSourceVolume() corev1.Volume {
	return corev1.Volume{
		Name: names.VolumeName(releaseSourceName),
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func releaseSourceVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      names.VolumeName(releaseSourceName),
		MountPath: bpmconverter.VolumeRenderingDataMountPath,
	}
}
