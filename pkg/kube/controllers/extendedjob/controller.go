package extendedjob

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllersconfig"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new ExtendedJob controller and adds it to the Manager
func Add(log *zap.SugaredLogger, ctrConfig *controllersconfig.ControllersConfig, mgr manager.Manager) error {
	query := NewQuery(mgr.GetClient())
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
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	client, err := corev1client.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}
	podLogGetter := NewPodLogGetter(client)
	jobReconciler, err := NewJobReconciler(log, ctrConfig, mgr, podLogGetter)
	jobController, err := controller.New("extendedjob-job-controller", mgr, controller.Options{Reconciler: jobReconciler})
	if err != nil {
		return err
	}
	predicate := predicate.Funcs{
		// We're only interested in Jobs going from Active to final state (Succeeded or Failed)
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.(*batchv1.Job).Status.Succeeded == 1 || e.ObjectNew.(*batchv1.Job).Status.Failed == 1
		},
	}
	err = jobController.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForObject{}, predicate)
	if err != nil {
		return err
	}

	return nil
}

// isJobPod matches our job pods
func isJobPod(labels map[string]string) bool {
	if _, exists := labels["job-name"]; exists {
		return true
	}
	return false
}
