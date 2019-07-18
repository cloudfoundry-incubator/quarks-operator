package versionedsecretstore

import corev1 "k8s.io/api/core/v1"

// GetConfigNamesFromSpec parses the owner object and returns two sets,
// the first containing the names of all referenced ConfigMaps,
// the second containing the names of all referenced Secrets
func GetConfigNamesFromSpec(spec corev1.PodSpec) (map[string]struct{}, map[string]struct{}) {
	// Create sets for storing the names fo the ConfigMaps/Secrets
	configMaps := make(map[string]struct{})
	secrets := make(map[string]struct{})

	// Iterate over all Volumes and check the VolumeSources for ConfigMaps
	// and Secrets
	for _, vol := range spec.Volumes {
		if cm := vol.VolumeSource.ConfigMap; cm != nil {
			configMaps[cm.Name] = struct{}{}
		}
		if s := vol.VolumeSource.Secret; s != nil {
			secrets[s.SecretName] = struct{}{}
		}
	}

	// Iterate over all Containers and their respective EnvFrom and Env
	// then check the EnvFromSources for ConfigMaps and Secrets
	for _, container := range spec.Containers {
		for _, env := range container.EnvFrom {
			if cm := env.ConfigMapRef; cm != nil {
				configMaps[cm.Name] = struct{}{}
			}
			if s := env.SecretRef; s != nil {
				secrets[s.Name] = struct{}{}
			}
		}

		for _, env := range container.Env {
			if env.ValueFrom == nil {
				continue
			}

			if cmRef := env.ValueFrom.ConfigMapKeyRef; cmRef != nil {
				configMaps[cmRef.Name] = struct{}{}

			}
			if sRef := env.ValueFrom.SecretKeyRef; sRef != nil {
				secrets[sRef.Name] = struct{}{}

			}
		}
	}

	return configMaps, secrets
}
