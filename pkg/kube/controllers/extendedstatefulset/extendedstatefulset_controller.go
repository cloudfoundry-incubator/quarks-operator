package extendedstatefulset

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	appsv1beta2client "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/reference"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// AddExtendedStatefulSet creates a new ExtendedStatefulSet controller to watch for the custom resource and
// reconcile it into statefulsets.
func AddExtendedStatefulSet(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-statefulset-reconciler", mgr.GetEventRecorderFor("ext-statefulset-recorder"))
	store := vss.NewVersionedSecretStore(mgr.GetClient())
	r := NewReconciler(ctx, config, mgr, controllerutil.SetControllerReference, store)

	// Create a new controller
	c, err := controller.New("ext-statefulset-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxExtendedStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding Extendedstatefulset controller to manager failed.")
	}

	client, err := appsv1beta2client.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}

	// Watch for changes to primary resource ExtendedStatefulSet
	// Trigger when
	// - create event of extendedStatefulSet which have no children resources
	// - update event of extendedStatefulSet
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*estsv1.ExtendedStatefulSet)
			sts, err := listStatefulSetsFromAPIClient(ctx, client, o)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to list StatefulSets owned by ExtendedStatefulSet '%s': %s", o.Name, err)
			}
			if len(sts) == 0 {
				ctxlog.NewPredicateEvent(e.Object).Debug(
					ctx, e.Meta, "estsv1.ExtendedStatefulSet",
					fmt.Sprintf("Create predicate passed for '%s'", e.Meta.GetName()),
				)
				return true
			}

			return false
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*estsv1.ExtendedStatefulSet)
			n := e.ObjectNew.(*estsv1.ExtendedStatefulSet)
			if !reflect.DeepEqual(o.Spec, n.Spec) {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, "estsv1.ExtendedStatefulSet",
					fmt.Sprintf("Update predicate passed for '%s'", e.MetaNew.GetName()),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &estsv1.ExtendedStatefulSet{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	// Watch ConfigMaps referenced by the ExtendedStatefulSet
	configMapPredicates := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConfigMap := e.ObjectOld.(*corev1.ConfigMap)
			newConfigMap := e.ObjectNew.(*corev1.ConfigMap)

			return !reflect.DeepEqual(oldConfigMap.Data, newConfigMap.Data)
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			config := a.Object.(*corev1.ConfigMap)

			if reference.SkipReconciles(ctx, mgr.GetClient(), config) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedStatefulSet, config, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for configMap '%s': %v", config.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "ExtendedStatefulSet", a.Meta.GetName(), "config-maps")
			}
			return reconciles
		}),
	}, configMapPredicates)
	if err != nil {
		return errors.Wrapf(err, "Watching configmaps failed in Extendedstatefulset controller failed.")
	}

	// Watch Secrets referenced by the ExtendedStatefulSet
	secretPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {

			o := e.Object.(*corev1.Secret)
			shouldProcessEvent := vss.IsVersionedSecret(*o)
			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedStatefulSet, o, true)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", o.Name, err)
			}

			if shouldProcessEvent && len(reconciles) > 0 {
				ctxlog.NewPredicateEvent(e.Object).Debug(
					ctx, e.Meta, "estsv1.ExtendedStatefulSet",
					fmt.Sprintf("Create predicate passed for '%s'", e.Meta.GetName()),
				)
				return true
			}

			return false

		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret := e.ObjectOld.(*corev1.Secret)
			newSecret := e.ObjectNew.(*corev1.Secret)

			return !reflect.DeepEqual(oldSecret.Data, newSecret.Data)
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			secret := a.Object.(*corev1.Secret)

			if reference.SkipReconciles(ctx, mgr.GetClient(), secret) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedStatefulSet, secret, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", secret.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "ExtendedStatefulSet", a.Meta.GetName(), "secret")
			}

			return reconciles
		}),
	}, secretPredicates)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in Extendedstatefulset controller failed.")
	}

	return nil
}
