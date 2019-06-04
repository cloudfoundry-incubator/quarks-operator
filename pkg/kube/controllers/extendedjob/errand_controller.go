package extendedjob

import (
	"context"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/reference"
)

// AddErrand creates a new ExtendedJob controller to start errands when their
// trigger strategy matches
func AddErrand(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	f := controllerutil.SetControllerReference
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-job-errand-reconciler", mgr.GetRecorder("ext-job-errand-recorder"))
	r := NewErrandReconciler(ctx, config, mgr, f)
	c, err := controller.New("ext-job-errand-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Trigger when
	//  * errand jobs are to be run (Spec.Run changes from `manual` to `now` or the job is created with `now`)
	//  * auto-errands with UpdateOnConfigChange == true have changed config references
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			eJob := e.Object.(*ejv1.ExtendedJob)
			shouldProcessEvent := eJob.Spec.Trigger.Strategy == ejv1.TriggerNow || eJob.Spec.Trigger.Strategy == ejv1.TriggerOnce
			if shouldProcessEvent {
				ctxlog.WithEvent(eJob, "Predicates").Debugf(ctx,
					"Errand %s creation allowed. ExtendedJob trigger strategy, matches.",
					eJob.Name)
			}

			return shouldProcessEvent
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*ejv1.ExtendedJob)
			n := e.ObjectNew.(*ejv1.ExtendedJob)

			enqueueForManualErrand := n.Spec.Trigger.Strategy == ejv1.TriggerNow && o.Spec.Trigger.Strategy == ejv1.TriggerManual

			// enqueuing for auto-errand when referenced secrets changed
			enqueueForConfigChange := n.IsAutoErrand() && n.Spec.UpdateOnConfigChange && hasConfigsChanged(o, n)

			shouldProcessEvent := enqueueForManualErrand || enqueueForConfigChange
			if shouldProcessEvent {
				ctxlog.WithEvent(n, "Predicates").Debugf(ctx,
					"Errand %s update allowed. ExtendedJob trigger strategy, matches.",
					n.Name)
			}

			return shouldProcessEvent
		},
	}

	err = c.Watch(&source.Kind{Type: &ejv1.ExtendedJob{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	// Watch config maps referenced by resource ExtendedJob,
	// trigger auto errand if UpdateOnConfigChange=true and config data changed
	p = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*corev1.ConfigMap)
			n := e.ObjectNew.(*corev1.ConfigMap)
			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedJob, n)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for configMap '%s': %s", n.Name, err)
			}
			shouldProcessEvent := len(reconciles) > 0 && !reflect.DeepEqual(o.Data, n.Data)
			if shouldProcessEvent {
				ctxlog.WithEvent(n, "Predicates").Debugf(ctx,
					"Configmap %s update allowed. Configmap data has changed.",
					n.Name)
			}
			return shouldProcessEvent
		},
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			cm := a.Object.(*corev1.ConfigMap)

			if skipCMReconciles(ctx, mgr.GetClient(), cm) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedJob, cm)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for config '%s': %v", cm.Name, err)
			}

			return reconciles
		}),
	}, p)
	if err != nil {
		return err
	}

	// Watch secrets referenced by resource ExtendedJob
	// trigger auto errand if UpdateOnConfigChange=true and config data changed
	p = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*corev1.Secret)
			n := e.ObjectNew.(*corev1.Secret)
			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedJob, n)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secrets '%s': %s", n.Name, err)
			}
			shouldProcessEvent := len(reconciles) > 0 && !reflect.DeepEqual(o.Data, n.Data)
			if shouldProcessEvent {
				ctxlog.WithEvent(n, "Predicates").Debugf(ctx,
					"Secret %s update allowed. Secret data has changed.",
					n.Name)
			}
			return shouldProcessEvent
		},
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			s := a.Object.(*corev1.Secret)

			if skipSecretReconciles(ctx, mgr.GetClient(), s) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedJob, s)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", s.Name, err)
			}

			return reconciles
		}),
	}, p)

	return err
}

// hasConfigsChanged return true if object's config references changed
func hasConfigsChanged(oldEJob, newEJob *ejv1.ExtendedJob) bool {
	oldConfigMaps, oldSecrets := owner.GetConfigNamesFromSpec(oldEJob.Spec.Template.Spec)
	newConfigMaps, newSecrets := owner.GetConfigNamesFromSpec(newEJob.Spec.Template.Spec)

	if reflect.DeepEqual(oldConfigMaps, newConfigMaps) && reflect.DeepEqual(oldSecrets, newSecrets) {
		return false
	}

	// For versioned secret, we only enqueue changes for higher version of secrets
	for newSecret := range newSecrets {
		secretPrefix := names.GetPrefixFromVersionedSecretName(newSecret)
		newVersion, err := names.GetVersionFromVersionedSecretName(newSecret)
		if err != nil {
			continue
		}

		if isLowerVersion(oldSecrets, secretPrefix, newVersion) {
			return false
		}
	}

	// other configs changes should be enqueued
	return true
}

func isLowerVersion(oldSecrets map[string]struct{}, secretPrefix string, newVersion int) bool {
	for oldSecret := range oldSecrets {
		if strings.HasPrefix(oldSecret, secretPrefix) {
			oldVersion, _ := names.GetVersionFromVersionedSecretName(oldSecret)

			if newVersion < oldVersion {
				return true
			}
		}
	}

	// if not found in old secrets, it's a new versioned secret
	return false
}

// skipCMReconciles gets the cm resource again. We want to skip if this is the mapAndEnqueue for ObjectOld
func skipCMReconciles(ctx context.Context, client client.Client, obj *corev1.ConfigMap) bool {
	n := &corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, n)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to get config map '%s': %s", obj.Name, err)
		return true
	}
	if obj.ObjectMeta.ResourceVersion != n.ObjectMeta.ResourceVersion {
		ctxlog.Debugf(ctx, "skip reconcile request for old resource version of '%s'", obj.Name)
		return true
	}
	return false
}

// skipSecretReconciles gets the secret resource again. We want to skip if this is the mapAndEnqueue for ObjectOld
func skipSecretReconciles(ctx context.Context, client client.Client, obj *corev1.Secret) bool {
	n := &corev1.Secret{}
	err := client.Get(ctx, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, n)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to get config map '%s': %s", obj.Name, err)
		return true
	}
	if obj.ObjectMeta.ResourceVersion != n.ObjectMeta.ResourceVersion {
		ctxlog.Debugf(ctx, "skip reconcile request for old resource version of '%s'", obj.Name)
		return true
	}
	return false
}
