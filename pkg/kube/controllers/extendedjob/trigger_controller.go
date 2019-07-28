package extendedjob

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// AddTrigger creates a new ExtendedJob controller and adds it to the Manager
func AddTrigger(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	query := NewQuery()
	f := controllerutil.SetControllerReference
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-job-trigger-reconciler", mgr.GetEventRecorderFor("ext-job-trigger-recorder"))
	store := vss.NewVersionedSecretStore(mgr.GetClient())
	r := NewTriggerReconciler(ctx, config, mgr, query, f, store)
	c, err := controller.New("ext-job-trigger-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return errors.Wrap(err, "Adding trigger controller to manager failed.")
	}

	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			if isJobPod(e.Meta.GetLabels()) {
				return false
			}
			// Can use this one to get to a new pod for our
			// notready and created states.  But this also triggers
			// on existing pods for controller restarts, so we only
			// want look at phase pending.
			pod := e.Object.(*corev1.Pod)
			shouldProcessEvent := pod.Status.Phase == "Pending"
			if shouldProcessEvent {
				ctxlog.NewPredicateEvent(pod).Debug(
					ctx, e.Meta, "corev1.Pod",
					fmt.Sprintf("Trigger eJob's create predicate passed for %s, existing pod is in Pending status", e.Meta.GetName()),
				)
			}
			return shouldProcessEvent
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// pod will be a 'not found' in reconciler, so skip
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// This allows matching both our ready and deleted states
			shouldProcessEvent := !isJobPod(e.MetaNew.GetLabels())

			if shouldProcessEvent {
				pod := e.ObjectOld.(*corev1.Pod)

				ctxlog.NewPredicateEvent(pod).Debug(
					ctx, e.MetaOld, "corev1.Pod",
					fmt.Sprintf("Trigger eJob's update predicate passed for %s, existing pod is in Pending status", e.MetaOld.GetName()),
				)
			}

			return shouldProcessEvent
		},
	}
	return c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, p)
}

// isJobPod matches our job pods
func isJobPod(labels map[string]string) bool {
	if _, exists := labels["job-name"]; exists {
		return true
	}
	return false
}
