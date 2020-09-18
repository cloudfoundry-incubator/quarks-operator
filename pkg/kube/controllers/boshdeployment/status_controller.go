package boshdeployment

import (
	"context"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"

	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/monitorednamespace"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AddBDPLStatusReconcilers creates a new BDPL Status controller to update BDPL status.
func AddBDPLStatusReconcilers(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "quarks-bdpl-status-reconciler", mgr.GetEventRecorderFor("quarks-bdpl-status-recorder"))
	r := NewStatusQSTSReconciler(ctx, config, mgr)
	rjobs := NewQJobStatusReconciler(ctx, config, mgr)

	// Create a new controller for qsts
	c, err := controller.New("quarks-bdpl-qsts-status-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding StatusQSTSReconciler controller to manager failed.")
	}

	// Create a new controller for qjobs
	cjobs, err := controller.New("quarks-bdpl-qjobs-status-controller", mgr, controller.Options{
		Reconciler:              rjobs,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding StatusQJobsReconciler controller to manager failed.")
	}

	nsPred := monitorednamespace.NewNSPredicate(ctx, mgr.GetClient(), config.MonitoredID)

	p := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return bdv1.HasDeploymentName(e.MetaNew.GetLabels()) || bdv1.HasDeploymentName(e.MetaOld.GetLabels())
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return bdv1.HasDeploymentName(e.Meta.GetLabels())
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &qstsv1a1.QuarksStatefulSet{}}, &handler.EnqueueRequestForObject{}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching QSTS in QuarksBDPLStatus controller failed.")
	}

	err = cjobs.Watch(&source.Kind{Type: &qjv1a1.QuarksJob{}}, &handler.EnqueueRequestForObject{}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching QJobs in QuarksBDPLStatus controller failed.")
	}

	return nil
}
