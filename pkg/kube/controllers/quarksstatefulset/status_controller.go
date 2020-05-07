package quarksstatefulset

import (
	"context"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	qstsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// AddQuarksStatefulSetStatus creates a new Status controller to update for quarks statefulset.
func AddQuarksStatefulSetStatus(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "quarks-statefulset-status-reconciler", mgr.GetEventRecorderFor("quarks-statefulset-status-recorder"))
	r := NewQuarksStatefulSetStatusReconciler(ctx, config, mgr)

	// Create a new controller
	c, err := controller.New("quarks-statefulset-status--controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding QuarksStatefulSetStatus controller to manager failed.")
	}

	// doamins are watched on updates too to get status changes
	certPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &qstsv1a1.QuarksStatefulSet{},
	}, certPred)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in QuarksStatefulSetStatus controller failed.")
	}

	return nil
}
