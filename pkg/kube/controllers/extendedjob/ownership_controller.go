package extendedjob

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
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

// Owner interface to manage ownership on configs and secrets
type Owner interface {
	Sync(context.Context, apis.Object, corev1.PodSpec) error
	RemoveOwnerReferences(context.Context, apis.Object, []apis.Object) error
	ListConfigsOwnedBy(context.Context, apis.Object) ([]apis.Object, error)
}

// AddOwnership creates a new ExtendedJob controller to update ownership on configs for auto errands.
func AddOwnership(log *zap.SugaredLogger, config *context.Config, mgr manager.Manager) error {
	l := log.Named("ext-job-owner-reconciler")
	owner := owner.NewOwner(mgr.GetClient(), l, mgr.GetScheme())
	r := NewOwnershipReconciler(l, config, mgr, controllerutil.SetControllerReference, owner)
	c, err := controller.New("ext-job-owner-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Only trigger if Spec.UpdateOnConfigChange is relevant
	p := predicate.Funcs{
		// Can't support the case where we are created with
		// `when: done`, because we would execute on every
		// controller restart.
		// TODO document that jobs created with done won't get triggered ever

		// Errand reconciler handles CreateEvent for `when: once`.
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*ejv1.ExtendedJob)
			n := e.ObjectNew.(*ejv1.ExtendedJob)
			return n.IsAutoErrand() && (updateOnConfigChanged(n, o) || n.ToBeDeleted())
		},
	}
	err = c.Watch(&source.Kind{Type: &ejv1.ExtendedJob{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	return nil
}

func updateOnConfigChanged(n, o *ejv1.ExtendedJob) bool {
	return (o.Spec.UpdateOnConfigChange == false && n.Spec.UpdateOnConfigChange == true) ||
		(o.Spec.UpdateOnConfigChange == true && n.Spec.UpdateOnConfigChange == false)
}
