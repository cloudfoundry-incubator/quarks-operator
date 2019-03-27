package extendedstatefulset

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// Add creates a new ExtendedStatefulSet controller and adds it to the Manager
func Add(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewReconcilerContext(ctx, "ext-statefulset-reconciler")
	r := NewReconciler(ctx, config, mgr, controllerutil.SetControllerReference)

	// Create a new controller
	c, err := controller.New("extendedstatefulset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ExtendedStatefulSet
	err = c.Watch(&source.Kind{Type: &essv1.ExtendedStatefulSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch ConfigMaps owned by resource ExtendedStatefulSet
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &essv1.ExtendedStatefulSet{},
	})
	if err != nil {
		return err
	}

	// Watch Secrets owned by resource ExtendedStatefulSet
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &essv1.ExtendedStatefulSet{},
	})
	if err != nil {
		return err
	}

	return nil
}
