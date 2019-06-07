package boshdeployment

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/reference"
)

// AddDeployment creates a new BOSHDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddDeployment(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "boshdeployment-reconciler", mgr.GetRecorder("boshdeployment-recorder"))
	r := NewDeploymentReconciler(
		ctx, config, mgr,
		bdm.NewResolver(mgr.GetClient(), func() bdm.Interpolator { return bdm.NewInterpolator() }),
		controllerutil.SetControllerReference,
	)

	// Create a new controller
	c, err := controller.New("boshdeployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource BOSHDeployment
	err = c.Watch(&source.Kind{Type: &bdv1.BOSHDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch ConfigMaps referenced by the BOSHDeployment
	configMapPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			configMap := e.Object.(*corev1.ConfigMap)

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, configMap)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for configMap '%s': %v", configMap.Name, err)
			}

			shouldProcessEvent := len(reconciles) > 0
			if shouldProcessEvent {
				for _, reconcile := range reconciles {
					ctxlog.WithEvent(configMap, "Predicates").Debugf(ctx,
						"Configmap %s creation allowed. Found reconciliation for BOSHDeployment in namespace %s.",
						configMap.Name,
						reconcile.Namespace)
				}
			}

			// The ConfigMap should reference at least one BOSHDeployment in order for us to consider it
			return shouldProcessEvent
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConfigMap := e.ObjectOld.(*corev1.ConfigMap)
			newConfigMap := e.ObjectNew.(*corev1.ConfigMap)

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, newConfigMap)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for configMap '%s': %v", newConfigMap.Name, err)
			}

			shouldProcessEvent := len(reconciles) > 0 && !reflect.DeepEqual(oldConfigMap.Data, newConfigMap.Data)
			if shouldProcessEvent {
				for _, reconcile := range reconciles {
					ctxlog.WithEvent(newConfigMap, "Predicates").Debugf(ctx,
						"Configmap %s update allowed. Found reconciliation for BOSHDeployment in namespace %s.",
						newConfigMap.Name,
						reconcile.Namespace)
				}
			}

			// The ConfigMap should reference at least one BOSHDeployment in order for us to consider it
			return shouldProcessEvent
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			config := a.Object.(*corev1.ConfigMap)

			if reference.SkipReconciles(ctx, mgr.GetClient(), config) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, config)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for config '%s': %v", config.Name, err)
			}

			return reconciles
		}),
	}, configMapPredicates)
	if err != nil {
		return err
	}

	// Watch Secrets referenced by the BOSHDeployment
	secretPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret := e.Object.(*corev1.Secret)
			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, secret)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", secret.Name, err)
			}

			shouldProcessEvent := len(reconciles) > 1
			if shouldProcessEvent {
				for _, reconcile := range reconciles {
					ctxlog.WithEvent(secret, "Predicates").Debugf(ctx,
						"Secret %s creation allowed. Found reconciliation for BOSHDeployment in namespace %s.",
						secret.Name,
						reconcile.Namespace)
				}

			}

			// The Secret should reference at least one BOSHDeployment in order for us to consider it
			return shouldProcessEvent
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret := e.ObjectOld.(*corev1.Secret)
			newSecret := e.ObjectNew.(*corev1.Secret)

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, newSecret)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", newSecret.Name, err)
			}

			shouldProcessEvent := len(reconciles) > 1 && !reflect.DeepEqual(oldSecret.Data, newSecret.Data)
			if shouldProcessEvent {
				for _, reconcile := range reconciles {
					ctxlog.WithEvent(newSecret, "Predicates").Debugf(ctx,
						"Secret %s update allowed. Found reconciliation for BOSHDeployment in namespace %s.",
						newSecret.Name,
						reconcile.Namespace)
				}
			}

			return shouldProcessEvent
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			secret := a.Object.(*corev1.Secret)

			if reference.SkipReconciles(ctx, mgr.GetClient(), secret) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, secret)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", secret.Name, err)
			}

			return reconciles
		}),
	}, secretPredicates)
	if err != nil {
		return err
	}

	return nil
}
