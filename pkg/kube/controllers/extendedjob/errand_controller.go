package extendedjob

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"

	"strings"

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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
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

	// Trigger when
	//  * errand jobs are to be run (Spec.Run changes from `manual` to `now` or the job is created with `now`)
	//  * auto-errands with UpdateOnConfigChange == true have changed config references
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			eJob := e.Object.(*ejv1.ExtendedJob)
			return eJob.Spec.Trigger.Strategy == ejv1.TriggerNow || eJob.Spec.Trigger.Strategy == ejv1.TriggerOnce
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

			enqueueForManualErrand := n.Spec.Trigger.Strategy == ejv1.TriggerNow && o.Spec.Trigger.Strategy == ejv1.TriggerManual

			// enqueuing for auto-errand when referenced secrets changed
			enqueueForConfigChange := n.IsAutoErrand() && n.Spec.UpdateOnConfigChange && hasConfigsChanged(o, n)
			return enqueueForManualErrand || enqueueForConfigChange
		},
	}

	// Only reconcile when
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

// hasConfigsChanged return true if object's config references changed
func hasConfigsChanged(oldEJob, newEJob *ejv1.ExtendedJob) bool {
	oldConfigMaps, oldSecrets := owner.GetConfigNamesFromSpec(oldEJob.Spec.Template.Spec)
	newConfigMaps, newSecrets := owner.GetConfigNamesFromSpec(newEJob.Spec.Template.Spec)

	if reflect.DeepEqual(oldConfigMaps, newConfigMaps) && reflect.DeepEqual(oldSecrets, newSecrets) {
		return false
	}

	// For versioned secret, we only enqueue changes for higher version of secrets
	for newSecret := range newSecrets {
		secretPrefix := names.GetPrefixFromVersionedSecretName(newSecret)
		newVersion, err := names.GetVersionFromVersionedSecretName(newSecret)
		if err != nil {
			continue
		}

		if isLowerVersion(oldSecrets, secretPrefix, newVersion) {
			return false
		}
	}

	// other configs changes should be enqueued
	return true
}

func isLowerVersion(oldSecrets map[string]struct{}, secretPrefix string, newVersion int) bool {
	for oldSecret := range oldSecrets {
		if strings.HasPrefix(oldSecret, secretPrefix) {
			oldVersion, _ := names.GetVersionFromVersionedSecretName(oldSecret)

			if newVersion < oldVersion {
				return true
			}
		}
	}

	// if not found in old secrets, it's a new versioned secret
	return false
}
