package reference

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/withops"
	"code.cloudfoundry.org/quarks-utils/pkg/podref"
)

// GetSecretsReferencedBy returns a list of all names for Secrets referenced by the object
// The object can be an QuarksStatefulSet or a BOSHDeployment
func getSecretsReferencedBy(ctx context.Context, client crc.Client, object interface{}) (map[string]bool, error) {
	switch object := object.(type) {
	case *bdv1.BOSHDeployment:
		return getSecretRefFromBdpl(ctx, client, *object)
	case *corev1.Pod:
		return podref.GetSecretRefFromPodSpec(object.Spec), nil
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
