package extendedjob

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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

// AddJob creates a new ExtendedJob controller and adds it to the Manager
func AddJob(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	client, err := corev1client.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}
	podLogGetter := NewPodLogGetter(client)
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-job-job-reconciler", mgr.GetRecorder("ext-job-job-recorder"))
	jobReconciler, err := NewJobReconciler(ctx, config, mgr, podLogGetter)
	if err != nil {
		return err
	}
	jobController, err := controller.New("ext-job-job-controller", mgr, controller.Options{Reconciler: jobReconciler})
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

			shouldProcessEvent := o.Status.Succeeded == 1 || o.Status.Failed == 1
			if shouldProcessEvent {
				ctxlog.WithPredicateEvent(o).DebugPredicate(
					ctx, e.MetaNew, "batchv1.Job",
					fmt.Sprintf("EJob job-output update predicate passed for %s, existing batchv1.Job has changed to a final state, either succeeded or failed",
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
