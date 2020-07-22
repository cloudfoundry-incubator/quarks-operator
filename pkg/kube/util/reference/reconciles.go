package reference

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	res "code.cloudfoundry.org/quarks-operator/pkg/kube/util/resources"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// ReconcileType lists all the types of reconciliations we can return,
// for controllers that have types that can reference ConfigMaps or Secrets
type ReconcileType int

const (
	// ReconcileForBOSHDeployment represents the BOSHDeployment CRD
	ReconcileForBOSHDeployment ReconcileType = iota
	// ReconcileForQuarksStatefulSet represents the QuarksStatefulSet CRD
	ReconcileForQuarksStatefulSet
	// ReconcileForPod represents the StatefulSet Kube Resource
	ReconcileForPod
)

func (r ReconcileType) String() string {
	return [...]string{
		"BOSHDeployment",
		"QuarksStatefulSet",
		"Pod",
	}[r]
}

// GetReconciles returns reconciliation requests for the BOSHDeployments or QuarksStatefulSets
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

	log.Debugf(ctx, "Listing '%s' for '%s/%s'", reconcileType, namespace, object.GetName())
	switch reconcileType {
	case ReconcileForBOSHDeployment:
		boshDeployments, err := res.ListBOSHDeployments(ctx, client, namespace)
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
	case ReconcileForQuarksStatefulSet:
		quarksStatefulSets, err := res.ListQuarksStatefulSets(ctx, client, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list QuarksStatefulSets for ConfigMap reconciles")
		}

		for _, quarksStatefulSet := range quarksStatefulSets.Items {
			if !quarksStatefulSet.Spec.UpdateOnConfigChange {
				continue
			}

			isRef, err := objReferencedBy(quarksStatefulSet)
			if err != nil {
				return nil, err
			}

			if isRef {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      quarksStatefulSet.Name,
						Namespace: quarksStatefulSet.Namespace,
					}})
			}
		}
	case ReconcileForPod:
		pods, err := res.ListPods(ctx, client, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list Pods for ConfigMap reconciles")
		}

		for _, pod := range pods.Items {
			isRef, err := objReferencedBy(pod)
			if err != nil {
				return nil, err
			}

			if isRef {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					}})
			}
		}
	default:
		return nil, fmt.Errorf("unknown reconcile type %s", reconcileType.String())
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
			log.Errorf(ctx, "Failed to get ConfigMap '%s/%s': %s", object.Namespace, object.Name, err)
			return true
		}

		newResourceVersion = cm.ResourceVersion
	case *corev1.Secret:
		s := &corev1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Name: object.Name, Namespace: object.Namespace}, s)
		if err != nil {
			log.Errorf(ctx, "Failed to get Secret '%s/%s': %s", object.Namespace, object.Name, err)
			return true
		}

		newResourceVersion = s.ResourceVersion
	default:
		return false
	}

	if object.GetResourceVersion() != newResourceVersion {
		log.Debugf(ctx, "Skipping reconcile request for old resource version of '%s/%s'", object.GetNamespace(), object.GetName())
		return true
	}
	return false
}
