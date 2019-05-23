package reference

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejobv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// ReconcileType lists all the types of reconciliations we can return,
// for controllers that have types that can reference ConfigMaps or Secrets
type ReconcileType int

const (
	// ReconcileForBOSHDeployment represents the BOSHDeployment CRD
	ReconcileForBOSHDeployment ReconcileType = iota
	// ReconcileForExtendedJob represents the ExtendedJob CRD
	ReconcileForExtendedJob
	// ReconcileForExtendedStatefulSet represents the ExtendedStatefulSet CRD
	ReconcileForExtendedStatefulSet
)

func (r ReconcileType) String() string {
	return [...]string{
		"BOSHDeployment",
		"ExtendedJob",
		"ExtendedSecret",
	}[r]
}

// GetReconciles returns reconciliation requests for the BOSHDeployments, ExtendedJobs or ExtendedStatefulSets
// that reference an object. The object can be a ConfigMap or a Secret
func GetReconciles(ctx context.Context, mgr manager.Manager, reconcileType ReconcileType, object interface{}, namespace string) ([]reconcile.Request, error) {
	isReferenceFor := func(parent interface{}) (bool, error) {
		var objectReferences map[string]bool
		var err error
		var name string

		switch object.(type) {
		case *corev1.ConfigMap:
			objectReferences, err = GetConfigMapsReferencedBy(parent)
			name = object.(*corev1.ConfigMap).Name
		case *corev1.Secret:
			objectReferences, err = GetSecretsReferencedBy(parent)
			name = object.(*corev1.Secret).Name
		default:
			return false, errors.New("can't get reconciles for unknown object type; supported types are ConfigMap and Secret")
		}

		if err != nil {
			return false, errors.Wrap(err, "error listing references")
		}

		_, ok := objectReferences[name]

		// TODO: also cover versioned secrets/configmaps

		return ok, nil
	}

	result := []reconcile.Request{}

	switch reconcileType {
	case ReconcileForBOSHDeployment:
		log.Debugf(ctx, "Listing BOSHDeployments in namespace '%s'", namespace)
		boshDeployments, err := listBOSHDeployments(ctx, mgr, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list BOSHDeployments for ConfigMap reconciles")
		}

		for _, boshDeployment := range boshDeployments.Items {
			isRef, err := isReferenceFor(boshDeployment)
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
	case ReconcileForExtendedJob:
		log.Debugf(ctx, "Listing ExtendedJobs in namespace '%s'", namespace)
		extendedJobs, err := listExtendedJobs(ctx, mgr, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list ExtendedJobs for ConfigMap reconciles")
		}

		for _, extendedJob := range extendedJobs.Items {
			isRef, err := isReferenceFor(extendedJob)
			if err != nil {
				return nil, err
			}

			if isRef {
				result = append(result, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      extendedJob.Name,
						Namespace: extendedJob.Namespace,
					}})
			}
		}
	case ReconcileForExtendedStatefulSet:
		log.Debugf(ctx, "Listing ExtendedStatefulSets in namespace '%s'", namespace)
		extendedStatefulSets, err := listExtendedStatefulSets(ctx, mgr, namespace)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list ExtendedStatefulSets for ConfigMap reconciles")
		}

		for _, extendedStatefulSet := range extendedStatefulSets.Items {
			isRef, err := isReferenceFor(extendedStatefulSet)
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

func listBOSHDeployments(ctx context.Context, mgr manager.Manager, namespace string) (*bdv1.BOSHDeploymentList, error) {
	log.Debugf(ctx, "Listing BOSHDeployments in namespace '%s'", namespace)
	result := &bdv1.BOSHDeploymentList{}
	err := mgr.GetClient().List(ctx, &client.ListOptions{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list BOSHDeployments")
	}

	return result, nil
}

func listExtendedStatefulSets(ctx context.Context, mgr manager.Manager, namespace string) (*estsv1.ExtendedStatefulSetList, error) {
	log.Debugf(ctx, "Listing ExtendedStatefulSets in namespace '%s'", namespace)
	result := &estsv1.ExtendedStatefulSetList{}
	err := mgr.GetClient().List(ctx, &client.ListOptions{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list ExtendedStatefulSets")
	}

	return result, nil
}

func listExtendedJobs(ctx context.Context, mgr manager.Manager, namespace string) (*ejobv1.ExtendedJobList, error) {
	log.Debugf(ctx, "Listing ExtendedJobs in namespace '%s'", namespace)
	result := &ejobv1.ExtendedJobList{}
	err := mgr.GetClient().List(ctx, &client.ListOptions{}, result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list ExtendedJobs")
	}

	return result, nil
}
