package boshdeployment

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	ctxlog "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// AddDeployment creates a new BOSHDeployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddDeployment(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "boshdeployment-reconciler", mgr.GetRecorder("boshdeployment-recorder"))
	r := NewReconciler(ctx, config, mgr, bdm.NewResolver(mgr.GetClient(), func() bdm.Interpolator { return bdm.NewInterpolator() }), controllerutil.SetControllerReference)

	// Create a new controller
	c, err := controller.New("boshdeployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource BOSHDeployment
	err = c.Watch(&source.Kind{Type: &bdv1.BOSHDeployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner BOSHDeployment
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &bdv1.BOSHDeployment{},
	})
	if err != nil {
		return err
	}

	// Watch ConfigMaps owned by resource BOSHDeployment
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &bdv1.BOSHDeployment{},
	})
	if err != nil {
		return err
	}

	// Watch Secrets owned by resource BOSHDeployment
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &bdv1.BOSHDeployment{},
	})
	if err != nil {
		return err
	}

	return nil
}
