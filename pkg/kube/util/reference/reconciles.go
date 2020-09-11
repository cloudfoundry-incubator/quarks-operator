package reference

import (
	"context"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// GetReconcilesForPod returns reconciliation requests for the BOSHDeployments or QuarksStatefulSets
// that reference an object. The object can be a ConfigMap or a Secret, it accepts an admit function which is used for filtering the object
func GetReconcilesForPod(ctx context.Context, client crc.Client, object apis.Object, admitFn func(v corev1.Pod) bool) ([]reconcile.Request, error) {
	namespace := object.GetNamespace()
	reconciles := []reconcile.Request{}

	log.Debugf(ctx, "Searching 'pods' for references to '%s/%s'", namespace, object.GetName())
	pods := &corev1.PodList{}
	err := client.List(ctx, pods, crc.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list Pods for ConfigMap reconciles")
	}

	for _, pod := range pods.Items {
		isRef, err := references(ctx, client, &pod, object)
		if err != nil {
			return nil, err
		}

		if isRef && admitFn(pod) {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				}})
		}
	}

	return reconciles, nil
}

// GetReconcilesForBDPL returns reconciliation requests for the BOSHDeployments or QuarksStatefulSets
// that reference an object. The object can be a ConfigMap or a Secret
func GetReconcilesForBDPL(ctx context.Context, client crc.Client, object apis.Object) ([]reconcile.Request, error) {
	namespace := object.GetNamespace()
	reconciles := []reconcile.Request{}

	log.Debugf(ctx, "Searching 'bdpls' for references to '%s/%s'", namespace, object.GetName())
	boshDeployments := &bdv1.BOSHDeploymentList{}
	err := client.List(ctx, boshDeployments, crc.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list BOSHDeployments for ConfigMap reconciles")
	}

	for _, boshDeployment := range boshDeployments.Items {
		isRef, err := references(ctx, client, &boshDeployment, object)
		if err != nil {
			return nil, err
		}

		if !isRef {
			continue
		}
		reconciles = append(reconciles, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      boshDeployment.Name,
				Namespace: boshDeployment.Namespace,
			}})
	}
	return reconciles, nil
}

// references returns true if parent uses object, e.g. as an env source
func references(ctx context.Context, client crc.Client, parent apis.Object, object apis.Object) (bool, error) {
	var (
		objectReferences map[string]bool
		err              error
		versionedSecret  bool
	)

	switch object := object.(type) {
	case *corev1.ConfigMap:
		objectReferences, err = getConfigMapsReferencedBy(parent)
	case *corev1.Secret:
		objectReferences, err = getSecretsReferencedBy(ctx, client, parent)
		versionedSecret = vss.IsVersionedSecret(*object)
	default:
		return false, errors.New("can't get reconciles for unknown object type; supported types are ConfigMap and Secret")
	}

	if err != nil {
		return false, errors.Wrap(err, "error listing references")
	}

	if versionedSecret {
		keys := make([]string, len(objectReferences))
		i := 0
		for k := range objectReferences {
			keys[i] = k
			i++
		}
		ok := vss.ContainsSecretName(keys, object.GetName())
		return ok, nil
	}

	_, ok := objectReferences[object.GetName()]
	return ok, nil
}
