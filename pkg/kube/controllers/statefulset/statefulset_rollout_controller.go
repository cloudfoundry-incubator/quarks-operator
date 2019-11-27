package statefulset

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// AddStatefulSetRollout creates a new statefulset rollout controller and adds it to the manager.
// The purpose of this controller is to remove the partition of the statefulset if the canary succeeds.
func AddStatefulSetRollout(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "statefulset-rollout-reconciler", mgr.GetEventRecorderFor("statefulset-rollout-recorder"))
	r := NewStatefulSetRolloutReconciler(ctx, config, mgr)

	// Create a new controller
	c, err := controller.New("statefulset-rollout-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding StatefulSet rollout controller to manager failed.")
	}

	// Trigger when annotation is set
	statefulSetPredicates := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			if CheckUpdate(e) {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, "StatefulSet",
					fmt.Sprintf("Update predicate passed for '%s'", e.MetaNew.GetName()),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForObject{}, statefulSetPredicates)
	if err != nil {
		return errors.Wrapf(err, "Watching StatefulSet failed in StatefulSet rollout controller.")
	}

	return nil
}

// CheckUpdate checks if update event should be processed
func CheckUpdate(e event.UpdateEvent) bool {
	newSts := e.ObjectNew.(*appsv1.StatefulSet)
	state, ok := newSts.Annotations[AnnotationCanaryRollout]
	if !ok || state == rolloutStateDone || state == rolloutStateFailed {
		return false
	}
	if state == rolloutStatePending {
		return true
	}
	oldSts := e.ObjectOld.(*appsv1.StatefulSet)
	if oldSts.Status.ReadyReplicas == newSts.Status.ReadyReplicas &&
		oldSts.Status.UpdatedReplicas == newSts.Status.UpdatedReplicas &&
		oldSts.Status.Replicas == newSts.Status.Replicas {
		return false
	}
	return true

}
