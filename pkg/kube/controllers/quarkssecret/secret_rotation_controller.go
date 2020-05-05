package quarkssecret

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	qsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// AddSecretRotation resets all QuarksSecret to status' to generated=false
func AddSecretRotation(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "secret-rotation-reconciler", mgr.GetEventRecorderFor("quarks-secret-recorder"))
	r := NewSecretRotationReconciler(ctx, config, mgr)

	// Create a new controller
	c, err := controller.New("secret-rotation-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxQuarksSecretWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding quarks secret controller to manager failed.")
	}

	// Watch for changes to QuarksSecrets
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.ConfigMap)
			labels := o.GetLabels()
			_, found := labels[qsv1a1.LabelSecretRotationTrigger]
			if found {
				ctxlog.NewPredicateEvent(e.Object).Debug(
					ctx, e.Meta, "corev1.ConfigMap",
					fmt.Sprintf("Create predicate passed for '%s/%s'", e.Meta.GetNamespace(), e.Meta.GetName()),
				)
				return true
			}
			return false
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return errors.Wrapf(err, "Watching quarks secrets failed in quarksSecret controller.")
	}

	return nil
}
