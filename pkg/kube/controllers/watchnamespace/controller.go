package watchnamespace

import (
	"context"
	"log"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/quarks-utils/pkg/config"
)

// Reconcile implements reconcile.Reconciler
type Reconcile struct {
}

var _ reconcile.Reconciler = &Reconcile{}

// Reconcile doesn't do anything, because there is nothing to reconcile with the watched namespace
func (r *Reconcile) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

// AddTerminate terminates the operator if the watch namespace disappears
func AddTerminate(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	c, err := controller.New("watch-namespace-terminate-controller", mgr, controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler:              &Reconcile{},
	})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{Type: &corev1.Namespace{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool { return false },
			DeleteFunc: func(e event.DeleteEvent) bool {
				if e.Meta.GetName() == config.Namespace {
					log.Fatal("Watch namespace is going away! Terminating operator.")
				}
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool { return false },
			UpdateFunc:  func(e event.UpdateEvent) bool { return false },
		},
	)
	if err != nil {
		return err
	}

	return nil
}
