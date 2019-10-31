package reference

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// ReconcileType lists all the types of reconciliations we can return,
// for controllers that have types that can reference ConfigMaps or Secrets
type ReconcileType int

const (
	// ReconcileForBOSHDeployment represents the BOSHDeployment CRD
	ReconcileForBOSHDeployment ReconcileType = iota
	// ReconcileForExtendedStatefulSet represents the ExtendedStatefulSet CRD
	ReconcileForExtendedStatefulSet
)

func (r ReconcileType) String() string {
	return [...]string{
		"BOSHDeployment",
		"ExtendedStatefulSet",
	}[r]
}

// GetReconciles returns reconciliation requests for the BOSHDeployments or ExtendedStatefulSets
// that reference an object. The object can be a ConfigMap or a Secret
func GetReconciles(ctx context.Context, client crc.Client, reconcileType ReconcileType, object apis.Object, versionCheck bool) ([]reconcile.Request, error) {
	objReferencedBy := func(parent interface{}) (bool, error) {
		var (
			objectReferences map[string]bool
			err              error
			name             string
			versionedSecret  bool
		)

		switch object := object.(type) {
		case *corev1.ConfigMap:
			objectReferences, err = GetConfigMapsReferencedBy(parent)
			name = object.Name
		case *corev1.Secret:
			objectReferences, err = GetSecretsReferencedBy(ctx, client, parent)
			name = object.Name
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
			ok := vss.ContainsSecretName(keys, name)
			if versionCheck && ok {
				ok := vss.ContainsOutdatedSecretVersion(keys, name)
				return ok, nil
			}
			return ok, nil
		}

		_, ok := objectReferences[name]
		return ok, nil
	}

	namespace := object.GetNamespace()
	result := []reconcile.Request{}

	switch reconcileType {
	case ReconcileForBOSHDeployment:
		boshDeployments, err := listBOSHDeployments(ctx, client, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list BOSHDeployments for ConfigMap reconciles")
		}

		for _, boshDeployment := range boshDeployments.Items {
			isRef, err := objReferencedBy(boshDeployment)
			if err != nil {
				return nil, err
			}

			if isRef {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      boshDeployment.Name,
						Namespace: boshDeployment.Namespace,
					}})
			}
		}
	case ReconcileForExtendedStatefulSet:
		extendedStatefulSets, err := listExtendedStatefulSets(ctx, client, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list ExtendedStatefulSets for ConfigMap reconciles")
		}

		for _, extendedStatefulSet := range extendedStatefulSets.Items {
			if !extendedStatefulSet.Spec.UpdateOnConfigChange {
				continue
			}

			isRef, err := objReferencedBy(extendedStatefulSet)
			if err != nil {
				return nil, err
			}

			if isRef {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      extendedStatefulSet.Name,
						Namespace: extendedStatefulSet.Namespace,
					}})
			}
		}
	default:
		return nil, fmt.Errorf("unkown reconcile type %s", reconcileType.String())
	}

	return result, nil
}

// SkipReconciles returns true if the object is stale, and shouldn't be enqueued for reconciliation
// The object can be a ConfigMap or a Secret
func SkipReconciles(ctx context.Context, client crc.Client, object apis.Object) bool {
	var newResourceVersion string

	switch object := object.(type) {
	case *corev1.ConfigMap:
		cm := &corev1.ConfigMap{}
		err := client.Get(ctx, types.NamespacedName{Name: object.Name, Namespace: object.Namespace}, cm)
		if err != nil {
			log.Errorf(ctx, "Failed to get ConfigMap '%s': %s", object.Name, err)
			return true
		}

		newResourceVersion = cm.ResourceVersion
	case *corev1.Secret:
		s := &corev1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Name: object.Name, Namespace: object.Namespace}, s)
		if err != nil {
			log.Errorf(ctx, "Failed to get Secret '%s': %s", object.Name, err)
			return true
		}

		newResourceVersion = s.ResourceVersion
	default:
		return false
	}

	if object.GetResourceVersion() != newResourceVersion {
		log.Debugf(ctx, "skip reconcile request for old resource version of '%s'", object.GetName())
		return true
	}
	return false
}

func listBOSHDeployments(ctx context.Context, client crc.Client, namespace string) (*bdv1.BOSHDeploymentList, error) {
	log.Debugf(ctx, "Listing BOSHDeployments in namespace '%s'", namespace)
	result := &bdv1.BOSHDeploymentList{}
	err := client.List(ctx, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list BOSHDeployments")
	}

	return result, nil
}

func listExtendedStatefulSets(ctx context.Context, client crc.Client, namespace string) (*estsv1.ExtendedStatefulSetList, error) {
	log.Debugf(ctx, "Listing ExtendedStatefulSets in namespace '%s'", namespace)
	result := &estsv1.ExtendedStatefulSetList{}
	err := client.List(ctx, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list ExtendedStatefulSets")
	}

	return result, nil
}
