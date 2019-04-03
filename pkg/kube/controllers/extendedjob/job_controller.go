package extendedjob

import (
	"context"

	"github.com/pkg/errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

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
	ctx = ctxlog.NewReconcilerContext(ctx, "ext-statefulset-reconciler")
	jobReconciler, err := NewJobReconciler(ctx, config, mgr, podLogGetter)
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
			if !e.ObjectNew.(*batchv1.Job).GetDeletionTimestamp().IsZero() {
				return false
			}
			if !isExtJobJob(e.MetaNew.GetLabels()) {
				return false
			}
			return e.ObjectNew.(*batchv1.Job).Status.Succeeded == 1 || e.ObjectNew.(*batchv1.Job).Status.Failed == 1
		},
	}
	return jobController.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForObject{}, predicate)
}

// isExtJobJob matches our jobs
func isExtJobJob(labels map[string]string) bool {
	if _, exists := labels["extendedjob"]; exists {
		return true
	}
	return false
}
