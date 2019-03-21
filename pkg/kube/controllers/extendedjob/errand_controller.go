package extendedjob

import (
	"reflect"

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

// AddErrand creates a new ExtendedJob controller to start errands when their
// trigger strategy matches
func AddErrand(log *zap.SugaredLogger, config *context.Config, mgr manager.Manager) error {
	f := controllerutil.SetControllerReference
	l := log.Named("ext-job-errand-reconciler")
	owner := owner.NewOwner(mgr.GetClient(), l, mgr.GetScheme())
	r := NewErrandReconciler(l, config, mgr, f, owner)
	c, err := controller.New("ext-job-errand-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}
	// Only trigger if Spec.Run is 'now'
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			exJob := e.Object.(*ejv1.ExtendedJob)
			return exJob.Spec.Trigger.Strategy == ejv1.TriggerNow || exJob.Spec.Trigger.Strategy == ejv1.TriggerOnce
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*ejv1.ExtendedJob)
			n := e.ObjectNew.(*ejv1.ExtendedJob)
			reconcile := n.Spec.Trigger.Strategy == ejv1.TriggerNow && o.Spec.Trigger.Strategy == ejv1.TriggerManual
			return reconcile
		},
	}
	err = c.Watch(&source.Kind{Type: &ejv1.ExtendedJob{}}, &handler.EnqueueRequestForObject{}, p)

	// Watch ConfigMaps owned by ExtendedJob, works because only auto
	// errands with UpdateOnConfigChange=true own configs
	p = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*corev1.ConfigMap)
			n := e.ObjectNew.(*corev1.ConfigMap)
			reconcile := !reflect.DeepEqual(o.Data, n.Data)
			return reconcile
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &ejv1.ExtendedJob{},
	}, p)
	if err != nil {
		return err
	}

	// Watch Secrets owned by resource ExtendedJob
	p = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*corev1.Secret)
			n := e.ObjectNew.(*corev1.Secret)
			reconcile := !reflect.DeepEqual(o.Data, n.Data)
			return reconcile
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &ejv1.ExtendedJob{},
	}, p)
	if err != nil {
		return err
	}

	return err
}
