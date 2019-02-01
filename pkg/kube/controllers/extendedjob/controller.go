package extendedjob

import (
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Add creates a new ExtendedJob controller and adds it to the Manager
func Add(log *zap.SugaredLogger, mgr manager.Manager) error {
	query := NewQuery(mgr.GetClient())
	f := controllerutil.SetControllerReference
	r := NewTriggerReconciler(log, mgr, query, f)
	c, err := controller.New("extendedjob-trigger-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}
