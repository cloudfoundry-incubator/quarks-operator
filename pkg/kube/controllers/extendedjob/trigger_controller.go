package extendedjob

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AddTrigger creates a new ExtendedJob controller and adds it to the Manager
func AddTrigger(log *zap.SugaredLogger, ctrConfig *context.Config, mgr manager.Manager) error {
	query := NewQuery(mgr.GetClient(), log)
	f := controllerutil.SetControllerReference
	r := NewTriggerReconciler(log, ctrConfig, mgr, query, f)
	c, err := controller.New("extendedjob-trigger-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
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
			test := pod.Status.Phase == "Pending"
			return test
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// pod will be a 'not found' in reconciler, so skip
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			if isJobPod(e.MetaNew.GetLabels()) {
				return false
			}
			// This allows matching both our ready and deleted states
			return true
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
