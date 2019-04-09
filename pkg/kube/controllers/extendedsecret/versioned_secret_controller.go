package extendedsecret

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// AddVersionedSecret creates a new controller that reconciles versioned secrets
func AddVersionedSecret(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewReconcilerContext(ctx, "manifest-reconciler")

	r := NewVersionedSecretReconciler(ctx, config, mgr)

	// Create a new controller
	c, err := controller.New("manifest-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch new Secret which is versioned secret
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret := e.Object.(*corev1.Secret)
			// Only enqueuing versioned secret which has required labels
			secretLabels := labels.Set(secret.GetLabels())
			if secretLabels == nil {
				return false
			}
			kind, _ := secretLabels[LabelSecretKind]
			_, dependantJobExist := secretLabels[ejv1.LabelDependantJobName]
			_, dependantSecretExist := secretLabels[ejv1.LabelDependantSecretName]
			return kind == "versionedSecret" && dependantJobExist && dependantSecretExist
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	return err
}
