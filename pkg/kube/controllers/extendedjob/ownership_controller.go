package extendedjob

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	eowner "code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// Owner interface to manage ownership on configs and secrets
type Owner interface {
	Sync(context.Context, apis.Object, corev1.PodSpec) error
	RemoveOwnerReferences(context.Context, apis.Object, []apis.Object) error
	ListConfigsOwnedBy(context.Context, apis.Object) ([]apis.Object, error)
}

// AddOwnership creates a new ExtendedJob controller to update ownership on configs for auto errands.
func AddOwnership(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-job-owner-reconciler", mgr.GetRecorder("ext-job-owner-recorder"))
	owner := eowner.NewOwner(mgr.GetClient(), mgr.GetScheme())
	r := NewOwnershipReconciler(ctx, config, mgr, controllerutil.SetControllerReference, owner)
	c, err := controller.New("ext-job-owner-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Only trigger if Spec.UpdateOnConfigChange is relevant and it's an auto errand
	p := predicate.Funcs{
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

	// pick up new configs which are referenced by an extended job
	p = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.ConfigMap)

			reconcile, err := hasConfigReferences(ctx, mgr.GetClient(), *o)
			if err != nil {
				ctxlog.WithEvent(o, "QueryExtendedJobError").Errorf(ctx, "Failed to query extended jobs: %s", err)
			}

			return reconcile
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}

	mapConfigs := handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
		configMap := a.Object.(*corev1.ConfigMap)
		return reconcilesForConfigMap(ctx, mgr, *configMap)
	})

	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: mapConfigs}, p)
	if err != nil {
		return err
	}

	// and for secrets
	p = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)

			// enqueuing secret referenced by EJob
			enqueueForReferencedSecret, err := hasSecretReferences(ctx, mgr.GetClient(), *o)
			if err != nil {
				ctxlog.WithEvent(o, "QueryExtendedJobError").Errorf(ctx, "Failed to query extended jobs: %s", err)
			}

			// enqueuing versioned secret which has required labels
			enqueueForVersionedSecret := hasVersionedSecretReferences(*o)

			return enqueueForReferencedSecret || enqueueForVersionedSecret
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}

	mapSecrets := handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
		secret := a.Object.(*corev1.Secret)
		return reconcilesForSecret(ctx, mgr, *secret)
	})

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: mapSecrets}, p)
	if err != nil {
		return err
	}

	return nil
}

func updateOnConfigChanged(n, o *ejv1.ExtendedJob) bool {
	return (!o.Spec.UpdateOnConfigChange && n.Spec.UpdateOnConfigChange) ||
		(o.Spec.UpdateOnConfigChange && !n.Spec.UpdateOnConfigChange)
}

// config name referenced by any ejob?
func hasConfigReferences(ctx context.Context, c client.Client, o corev1.ConfigMap) (bool, error) {
	eJobs := &ejv1.ExtendedJobList{}
	err := c.List(ctx, &client.ListOptions{}, eJobs)
	if err != nil {
		return false, err
	}

	if len(eJobs.Items) < 1 {
		return false, nil
	}

	for _, eJob := range eJobs.Items {
		configMapNames, _ := eowner.GetConfigNamesFromSpec(eJob.Spec.Template.Spec)
		if _, ok := configMapNames[o.GetName()]; ok {
			return true, nil
		}
	}

	return false, nil
}

// secret name referenced by any eJob?
func hasSecretReferences(ctx context.Context, c client.Client, o corev1.Secret) (bool, error) {
	eJobs := &ejv1.ExtendedJobList{}
	err := c.List(ctx, &client.ListOptions{}, eJobs)
	if err != nil {
		return false, err
	}

	if len(eJobs.Items) < 1 {
		return false, nil
	}

	for _, eJob := range eJobs.Items {
		_, secretNames := eowner.GetConfigNamesFromSpec(eJob.Spec.Template.Spec)
		if _, ok := secretNames[o.GetName()]; ok {
			return true, nil
		}
	}

	return false, nil
}

func hasVersionedSecretReferences(o corev1.Secret) bool {
	secretLabels := o.GetLabels()
	if secretLabels == nil {
		return false
	}
	kind := secretLabels[versionedsecretstore.LabelSecretKind]
	_, referencedJobExist := secretLabels[ejv1.LabelReferencedJobName]
	return kind == versionedsecretstore.VersionSecretKind && referencedJobExist
}

func reconcilesForConfigMap(ctx context.Context, mgr manager.Manager, configMap corev1.ConfigMap) []reconcile.Request {
	reconciles := []reconcile.Request{}

	eJobs := &ejv1.ExtendedJobList{}
	err := mgr.GetClient().List(ctx, &client.ListOptions{}, eJobs)
	if err != nil || len(eJobs.Items) < 1 {
		return reconciles
	}

	for _, eJob := range eJobs.Items {
		configMapNames, _ := eowner.GetConfigNamesFromSpec(eJob.Spec.Template.Spec)
		if _, ok := configMapNames[configMap.GetName()]; ok {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      eJob.GetName(),
					Namespace: eJob.GetNamespace(),
				},
			})
		}
	}

	return reconciles
}

func reconcilesForSecret(ctx context.Context, mgr manager.Manager, secret corev1.Secret) []reconcile.Request {
	reconciles := []reconcile.Request{}

	var referencedSecretName string
	var err error
	secretLabels := secret.GetLabels()
	if secretLabels != nil {
		referencedSecretName = names.GetPrefixFromVersionedSecretName(secret.GetName())
		if referencedSecretName == "" {
			return reconciles
		}
	}

	eJobs := &ejv1.ExtendedJobList{}
	err = mgr.GetClient().List(ctx, &client.ListOptions{}, eJobs)
	if err != nil || len(eJobs.Items) < 1 {
		return reconciles
	}

	for _, eJob := range eJobs.Items {
		_, secretNames := eowner.GetConfigNamesFromSpec(eJob.Spec.Template.Spec)
		// add requests for the ExtendedJob referencing the native secret
		if _, ok := secretNames[secret.GetName()]; ok {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      eJob.GetName(),
					Namespace: eJob.GetNamespace(),
				},
			})
		}
		// add requests for the ExtendedJob referencing the versioned secret
		for secretName := range secretNames {
			if strings.HasPrefix(secretName, referencedSecretName) {
				reconciles = append(reconciles, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      eJob.GetName(),
						Namespace: eJob.GetNamespace(),
					},
				})
			}
		}
	}

	return reconciles
}
