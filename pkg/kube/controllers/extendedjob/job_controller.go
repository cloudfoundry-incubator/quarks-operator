package extendedjob

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// AddJob creates a new Job controller to collect the output from jobs, persist
// that output as a secret and delete the k8s job afterwards.
func AddJob(ctx context.Context, config *config.Config, mgr manager.Manager) error {

	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-job-job-reconciler", mgr.GetEventRecorderFor("ext-job-job-recorder"))
	jobReconciler, err := NewJobReconciler(ctx, config, mgr)
	if err != nil {
		return err
	}
	jobController, err := controller.New("ext-job-job-controller", mgr, controller.Options{
		Reconciler:              jobReconciler,
		MaxConcurrentReconciles: config.MaxExtendedJobWorkers,
	})
	if err != nil {
		return err
	}
	predicate := predicate.Funcs{
		// We're only interested in Jobs going from Active to final state (Succeeded or Failed)
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectNew.(*batchv1.Job)
			if !o.GetDeletionTimestamp().IsZero() {
				return false
			}

			if !isEJobJob(e.MetaNew.GetLabels()) {
				return false
			}

			shouldProcessEvent := o.Status.Succeeded == 1
			if shouldProcessEvent {
				ctxlog.NewPredicateEvent(o).Debug(
					ctx, e.MetaNew, "batchv1.Job",
					fmt.Sprintf("Update predicate passed for '%s', existing batchv1.Job has changed to a final state, either succeeded or failed",
						e.MetaNew.GetName()),
				)
			}

			return shouldProcessEvent
		},
	}
	return jobController.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForObject{}, predicate)
}

// isEJobJob matches our jobs
func isEJobJob(labels map[string]string) bool {
	if _, exists := labels[ejv1.LabelExtendedJob]; exists {
		return true
	}
	return false
}
