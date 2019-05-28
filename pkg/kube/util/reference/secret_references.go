package reference

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejobv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
)

// GetSecretsReferencedBy returns a list of all names for Secrets referenced by the object
// The object can be an ExtendedStatefulSet, an ExtendedeJob or a BOSHDeployment
func GetSecretsReferencedBy(object interface{}) (map[string]bool, error) {
	// Figure out the type of object
	switch object := object.(type) {
	case bdv1.BOSHDeployment:
		return getSecretRefFromBdpl(object), nil
	case ejobv1.ExtendedJob:
		return getSecretRefFromEJob(object), nil
	case estsv1.ExtendedStatefulSet:
		return getSecretRefFromESts(object), nil
	default:
		return nil, errors.New("can't get secret references for unkown type; supported types are BOSHDeployment, ExtendedJob and ExtendedStatefulSet")
	}
}

func getSecretRefFromBdpl(object bdv1.BOSHDeployment) map[string]bool {
	result := map[string]bool{}

	if object.Spec.Manifest.Type == bdv1.SecretType {
		result[object.Spec.Manifest.Ref] = true
	}

	for _, ops := range object.Spec.Ops {
		if ops.Type == bdv1.SecretType {
			result[ops.Ref] = true
		}
	}

	return result
}

func getSecretRefFromESts(object estsv1.ExtendedStatefulSet) map[string]bool {
	return getSecretRefFromPod(object.Spec.Template.Spec.Template.Spec)
}

func getSecretRefFromEJob(object ejobv1.ExtendedJob) map[string]bool {
	return getSecretRefFromPod(object.Spec.Template.Spec)
}

func getSecretRefFromPod(object corev1.PodSpec) map[string]bool {
	result := map[string]bool{}

	// Look at all volumes
	for _, volume := range object.Volumes {
		if volume.VolumeSource.Secret != nil {
			result[volume.VolumeSource.Secret.SecretName] = true
		}
	}

	// Look at all init containers
	for _, container := range object.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				result[envFrom.SecretRef.Name] = true
			}
		}

		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
				result[envVar.ValueFrom.SecretKeyRef.Name] = true
			}
		}
	}

	// Look at all containers
	for _, container := range object.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.SecretRef != nil {
				result[envFrom.SecretRef.Name] = true
			}
		}

		for _, envVar := range container.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
				result[envVar.ValueFrom.SecretKeyRef.Name] = true
			}
		}
	}

	return result
}
