package quarksrestart

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/reference"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/monitorednamespace"
	"code.cloudfoundry.org/quarks-utils/pkg/skip"
)

// AnnotationRestartOnUpdate is the annotation required on the secret/configmap for the quarks restart feature
var AnnotationRestartOnUpdate = fmt.Sprintf("%s/restart-on-update", apis.GroupName)

const name = "quarks-restart"

// AddRestart creates a new controller to restart statefulsets,deployments & jobs
// if one of their pod's referred secrets/configmaps has changed
func AddRestart(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, name+"-reconciler", mgr.GetEventRecorderFor(name+"-recorder"))
	r := NewRestartReconciler(ctx, config, mgr)

	c, err := controller.New(name+"-controller", mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return errors.Wrap(err, "Adding restart controller to manager failed.")
	}

	nsPred := monitorednamespace.NewNSPredicate(ctx, mgr.GetClient(), config.MonitoredID)

	// watch secrets, trigger if one changes which is used by a pod
	p := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {

			oldSecret := e.ObjectOld.(*corev1.Secret)
			newSecret := e.ObjectNew.(*corev1.Secret)

			if !reflect.DeepEqual(oldSecret.Data, newSecret.Data) {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, "corev1.Secret",
					fmt.Sprintf("Update predicate passed for '%s/%s'", e.MetaNew.GetNamespace(), e.MetaNew.GetName()),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			secret := a.Object.(*corev1.Secret)

			if skip.Reconciles(ctx, mgr.GetClient(), secret) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconcilesWithFilter(ctx, mgr.GetClient(), reference.ReconcileForPod, secret, false, func(v interface{}) bool {
				pod := v.(corev1.Pod)
				annotations := pod.GetAnnotations()
				if _, found := annotations[AnnotationRestartOnUpdate]; !found {
					return false
				}
				return true
			})
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s/%s': %v", secret.Namespace, secret.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "RestartController", a.Meta.GetName(), "secret")
			}

			return reconciles
		}),
	}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in Restart controller failed.")
	}

	// watch configmaps, trigger if one changes which is used by a pod
	p = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {

			annotations := e.MetaNew.GetAnnotations()
			if _, found := annotations[AnnotationRestartOnUpdate]; !found {
				return false
			}

			oldConfigMap := e.ObjectOld.(*corev1.ConfigMap)
			newConfigMap := e.ObjectNew.(*corev1.ConfigMap)

			if !reflect.DeepEqual(oldConfigMap.Data, newConfigMap.Data) {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, "corev1.ConfigMap",
					fmt.Sprintf("Update predicate passed for '%s/%s'", e.MetaNew.GetNamespace(), e.MetaNew.GetName()),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			configmap := a.Object.(*corev1.ConfigMap)

			if skip.Reconciles(ctx, mgr.GetClient(), configmap) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForPod, configmap, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for configmap '%s/%s': %v", configmap.Namespace, configmap.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "RestartController", a.Meta.GetName(), "configmap")
			}

			return reconciles
		}),
	}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching configmaps failed in Restart controller failed.")
	}
	return nil
}
