package reference

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejobv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
)

// GetSecretsReferencedBy returns a list of all names for Secrets referenced by the object
// The object can be an ExtendedStatefulSet, an ExtendedJob or a BOSHDeployment
func GetSecretsReferencedBy(ctx context.Context, client crc.Client, object interface{}) (map[string]bool, error) {
	switch object := object.(type) {
	case bdv1.BOSHDeployment:
		return getSecretRefFromBdpl(ctx, client, object)
	case ejobv1.ExtendedJob:
		return getSecretRefFromEJob(object), nil
	case estsv1.ExtendedStatefulSet:
		return getSecretRefFromESts(object), nil
	default:
		return nil, errors.New("can't get secret references for unknown type; supported types are BOSHDeployment, ExtendedJob and ExtendedStatefulSet")
	}
}

func getSecretRefFromBdpl(ctx context.Context, client crc.Client, object bdv1.BOSHDeployment) (map[string]bool, error) {
	result := map[string]bool{}

	if object.Spec.Manifest.Type == bdv1.SecretReference {
		result[object.Spec.Manifest.Name] = true
	}

	for _, ops := range object.Spec.Ops {
		if ops.Type == bdv1.SecretReference {
			result[ops.Name] = true
		}
	}

	// Include secrets of implicit vars
	resolver := converter.NewResolver(client, func() converter.Interpolator { return converter.NewInterpolator() })
	_, implicitVars, err := resolver.WithOpsManifest(ctx, &object, object.Namespace)
	if err != nil {
		return map[string]bool{}, errors.Wrap(err, fmt.Sprintf("Failed to load the with-ops manifest for BOSHDeployment '%s/%s'", object.Namespace, object.Name))
	}
	for _, iv := range implicitVars {
		result[iv] = true
	}

	return result, nil
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
	for _, container := range object.InitContainers {
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
