package extendedjob

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// AddErrand creates a new ExtendedJob controller to start errands when their
// trigger strategy matches
func AddErrand(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	f := controllerutil.SetControllerReference
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-job-errand-reconciler", mgr.GetRecorder("ext-job-errand-recorder"))
	owner := owner.NewOwner(mgr.GetClient(), mgr.GetScheme())
	r := NewErrandReconciler(ctx, config, mgr, f, owner)
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
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			// Only enqueuing versioned secret which has versionedSecret label
			secretLabels := o.GetLabels()
			if secretLabels == nil {
				return false
			}

			if kind, ok := secretLabels[versionedsecretstore.LabelSecretKind]; ok && kind == versionedsecretstore.VersionSecretKind {
				return true
			}

			return false
		},
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
