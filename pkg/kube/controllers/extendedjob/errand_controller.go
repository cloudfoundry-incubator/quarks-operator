package extendedjob

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/reference"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
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
				ctxlog.WithEvent(eJob, "Predicates").DebugJSON(ctx,
					"ejob: create predicate for errand controller",
					ctxlog.ReconcileEventsFromSource{
						ReconciliationObjectName: e.Meta.GetName(),
						ReconciliationObjectKind: ejv1.LabelExtendedJob,
						PredicateObjectName:      e.Meta.GetName(),
						PredicateObjectKind:      ejv1.LabelExtendedJob,
						Namespace:                e.Meta.GetNamespace(),
						Type:                     "predicate",
						Message: fmt.Sprintf("Filter passed for %s, existing extendedJob spec.Trigger.Strategy  matches the values 'now' or 'once'",
							e.Meta.GetName()),
					},
				)
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
				ctxlog.WithEvent(o, "Predicates").DebugJSON(ctx,
					"ejob: update predicate for errand controller",
					ctxlog.ReconcileEventsFromSource{
						ReconciliationObjectName: e.MetaNew.GetName(),
						ReconciliationObjectKind: ejv1.LabelExtendedJob,
						PredicateObjectName:      e.MetaNew.GetName(),
						PredicateObjectKind:      ejv1.LabelExtendedJob,
						Namespace:                e.MetaNew.GetNamespace(),
						Type:                     "predicate",
						Message: fmt.Sprintf("Filter passed for %s, a change in it´s referenced secrets have been detected",
							e.MetaNew.GetName()),
					},
				)
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
			return len(reconciles) > 0 && !reflect.DeepEqual(o.Data, n.Data)
		},
	}

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			cm := a.Object.(*corev1.ConfigMap)

			if reference.SkipReconciles(ctx, mgr.GetClient(), cm) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedJob, cm)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for config '%s': %v", cm.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.WithEvent(a.Object, "Mapping").DebugJSON(ctx,
					"Enqueuing reconcile requests in response to events",
					ctxlog.ReconcileEventsFromSource{
						ReconciliationObjectName: reconciliation.Name,
						ReconciliationObjectKind: "ExtendedJob",
						PredicateObjectName:      a.Meta.GetName(),
						PredicateObjectKind:      bdv1.ConfigMapType,
						Namespace:                reconciliation.Namespace,
						Type:                     "mapping",
						Message:                  fmt.Sprintf("fan-out updates from %s, type %s into %s", a.Meta.GetName(), bdv1.ConfigMapType, reconciliation.Name),
					})
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
		// Only enqueuing versioned secret which has versionedSecret label
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			ok := vss.IsVersionedSecret(*o)
			// Skip initial version since it will trigger twice if the job has been created with
			// `Strategy: Once` and secrets are created afterwards
			if ok && vss.IsInitialVersion(*o) {
				return false
			}
			return ok
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		// React to updates on all referenced secrets
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*corev1.Secret)
			n := e.ObjectNew.(*corev1.Secret)
			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedJob, n)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secrets '%s': %s", n.Name, err)
			}
			return len(reconciles) > 0 && !reflect.DeepEqual(o.Data, n.Data)
		},
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			s := a.Object.(*corev1.Secret)

			if reference.SkipReconciles(ctx, mgr.GetClient(), s) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForExtendedJob, s)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s': %v", s.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.WithEvent(a.Object, "Mapping").DebugJSON(ctx,
					"Enqueuing reconcile requests in response to events",
					ctxlog.ReconcileEventsFromSource{
						ReconciliationObjectName: reconciliation.Name,
						ReconciliationObjectKind: "ExtendedJob",
						PredicateObjectName:      a.Meta.GetName(),
						PredicateObjectKind:      bdv1.SecretType,
						Namespace:                reconciliation.Namespace,
						Type:                     "mapping",
						Message:                  fmt.Sprintf("fan-out updates from %s, type %s into %s", a.Meta.GetName(), bdv1.SecretType, reconciliation.Name),
					})
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
		secretPrefix := vss.NamePrefix(newSecret)
		newVersion, err := vss.VersionFromName(newSecret)
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
			oldVersion, _ := vss.VersionFromName(oldSecret)

			if newVersion < oldVersion {
				return true
			}
		}
	}

	// if not found in old secrets, it's a new versioned secret
	return false
}
