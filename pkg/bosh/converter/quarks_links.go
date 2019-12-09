package converter

import corev1 "k8s.io/api/core/v1"

const (
	// VolumeLinksPath is the mount path for the links data.
	VolumeLinksPath = "/var/run/secrets/links/"
)

// LinkInfo specifies the link provider and its secret name from Kube native components
type LinkInfo struct {
	SecretName   string
	ProviderName string
}

// LinkInfos is a list of LinkInfo
type LinkInfos []LinkInfo

// Volumes returns a list of volumes from LinkInfos
func (q *LinkInfos) Volumes() []corev1.Volume {
	volumes := []corev1.Volume{}
	for _, l := range *q {
		volumes = append(volumes, l.linkVolume())
	}

	return volumes
}

// VolumeMounts returns a list of volumeMounts from LinkInfos
func (q *LinkInfos) VolumeMounts() []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{}
	for _, l := range *q {
		volumeMounts = append(volumeMounts, l.linkVolumeMount())
	}

	return volumeMounts
}

func (q *LinkInfo) linkVolume() corev1.Volume {
	return corev1.Volume{
		Name: generateVolumeName(q.SecretName),
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: q.SecretName,
			},
		},
	}
}

func (q *LinkInfo) linkVolumeMount() corev1.VolumeMount {
	return corev1.VolumeMount{
		Name:      generateVolumeName(q.SecretName),
		MountPath: VolumeLinksPath + q.ProviderName,
		ReadOnly:  true,
	}
}
