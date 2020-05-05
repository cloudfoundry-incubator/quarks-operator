package reference

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	qstsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/withops"
)

// GetSecretsReferencedBy returns a list of all names for Secrets referenced by the object
// The object can be an QuarksStatefulSet or a BOSHDeployment
func GetSecretsReferencedBy(ctx context.Context, client crc.Client, object interface{}) (map[string]bool, error) {
	switch object := object.(type) {
	case bdv1.BOSHDeployment:
		return getSecretRefFromBdpl(ctx, client, object)
	case qstsv1a1.QuarksStatefulSet:
		return getSecretRefFromESts(object), nil
	default:
		return nil, errors.New("can't get secret references for unknown type; supported types are BOSHDeployment and QuarksStatefulSet")
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

	for _, userVar := range object.Spec.Vars {
		result[userVar.Secret] = true
	}

	// Include secrets of implicit vars
	withops := withops.NewResolver(
		client,
		func() withops.Interpolator { return withops.NewInterpolator() },
		func(m bdm.Manifest) (withops.DomainNameService, error) {
			return boshdns.NewDNS(m)
		},
	)
	_, implicitVars, err := withops.Manifest(ctx, &object, object.Namespace)
	if err != nil {
		return map[string]bool{}, errors.Wrap(err, fmt.Sprintf("Failed to load the with-ops manifest for BOSHDeployment '%s/%s'", object.Namespace, object.Name))
	}
	for _, iv := range implicitVars {
		result[iv] = true
	}

	return result, nil
}

func getSecretRefFromESts(object qstsv1a1.QuarksStatefulSet) map[string]bool {
	return getSecretRefFromPod(object.Spec.Template.Spec.Template.Spec)
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
