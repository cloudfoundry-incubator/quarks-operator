package quarksstatefulset

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/reference"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// AddQuarksStatefulSet creates a new QuarksStatefulSet controller to watch for the custom resource and
// reconcile it into statefulSets.
func AddQuarksStatefulSet(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "quarks-statefulset-reconciler", mgr.GetEventRecorderFor("quarks-statefulset-recorder"))
	store := vss.NewVersionedSecretStore(mgr.GetClient())
	r := NewReconciler(ctx, config, mgr, controllerutil.SetControllerReference, store)

	// Create a new controller
	c, err := controller.New("quarks-statefulset-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding QuarksStatefulSet controller to manager failed.")
	}

	client, err := appsv1client.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}

	// Watch for changes to primary resource QuarksStatefulSet
	// Trigger when
	// - create event of quarksStatefulSet which have no children resources
	// - update event of quarksStatefulSet
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*qstsv1a1.QuarksStatefulSet)
			sts, err := listStatefulSetsFromAPIClient(ctx, client, o)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to list StatefulSets owned by QuarksStatefulSet '%s': %s", o.Name, err)
			}
			if len(sts) == 0 {
				ctxlog.NewPredicateEvent(e.Object).Debug(
					ctx, e.Meta, "qstsv1a1.QuarksStatefulSet",
					fmt.Sprintf("Create predicate passed for '%s'", e.Meta.GetName()),
				)
				return true
			}

			return false
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*qstsv1a1.QuarksStatefulSet)
			n := e.ObjectNew.(*qstsv1a1.QuarksStatefulSet)
			if !reflect.DeepEqual(o.Spec, n.Spec) {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, "qstsv1a1.QuarksStatefulSet",
					fmt.Sprintf("Update predicate passed for '%s'", e.MetaNew.GetName()),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &qstsv1a1.QuarksStatefulSet{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	// Watch ConfigMaps referenced by the QuarksStatefulSet
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

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForQuarksStatefulSet, config, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for configMap '%s': %v", config.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "QuarksStatefulSet", a.Meta.GetName(), "config-maps")
			}
			return reconciles
		}),
	}, configMapPredicates)
	if err != nil {
		return errors.Wrapf(err, "Watching configMaps failed in QuarksStatefulSet controller failed.")
	}

	// Watch Secrets referenced by the QuarksStatefulSet
	secretPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {

			o := e.Object.(*corev1.Secret)
			shouldProcessEvent := vss.IsVersionedSecret(*o)
			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForQuarksStatefulSet, o, true)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", o.Name, err)
			}

			if shouldProcessEvent && len(reconciles) > 0 {
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

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForQuarksStatefulSet, secret, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", secret.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "QuarksStatefulSet", a.Meta.GetName(), "secret")
			}

			return reconciles
		}),
	}, secretPredicates)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in QuarksStatefulSet controller failed.")
	}

	return nil
}
