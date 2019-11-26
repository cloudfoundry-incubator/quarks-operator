package quarksstatefulset

import (
	"context"
	"fmt"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// AddStatefulSetActivePassive creates a new QuarksStatefulSet controller that watches multiple instances
// of a specific resource, and decides which is active and which is passive
func AddStatefulSetActivePassive(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "active-passive-reconciler", mgr.GetEventRecorderFor("quarks-active-passive-recorder"))
	kubeConfig, _ := KubeConfig()

	kclient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return errors.Wrap(err, "failed retrieving kubernetes client configuration")
	}

	r := NewActivePassiveReconciler(ctx, config, mgr, kclient)

	// Create new controller
	c, err := controller.New("active-passive-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxQuarksStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "adding active-passive controller to manager failed")
	}

	stsPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			qo, ok := e.Object.(*qstsv1a1.QuarksStatefulSet)
			if !ok {
				return false
			}
			activePassiveCmd := qo.Spec.ActivePassiveProbe
			if activePassiveCmd != nil {
				ctxlog.NewPredicateEvent(e.Object).Debug(
					ctx, e.Meta, "qstsv1a1.QuarksStatefulSet",
					fmt.Sprintf("Create predicate passed for active-passive '%s'", e.Meta.GetName()),
				)
				return true
			}
			return false
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			newStatefulSet := e.ObjectNew.(*qstsv1a1.QuarksStatefulSet)

			activePassiveCmd := newStatefulSet.Spec.ActivePassiveProbe
			if activePassiveCmd != nil {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, "qstsv1a1.QuarksStatefulSet",
					fmt.Sprintf("Create predicate passed for active-passive '%s'", e.MetaNew.GetName()),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &qstsv1a1.QuarksStatefulSet{}},
		&handler.EnqueueRequestForObject{},
		stsPredicates)
	if err != nil {
		return errors.Wrapf(err, "watching QuarksStatefulSet failed in active/passive controller")
	}

	return nil
}
