package boshdeployment

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

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	ctxlog "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

// AddGeneratedVariable creates a new generated variable Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddGeneratedVariable(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "generated-variable-reconciler", mgr.GetRecorder("generated-variable-recorder"))
	r := NewGeneratedVariableReconciler(ctx, config, mgr, bdm.NewResolver(mgr.GetClient(), func() bdm.Interpolator { return bdm.NewInterpolator() }), controllerutil.SetControllerReference)

	// Create a new controller
	c, err := controller.New("generated-variable-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch Secrets of manifest with ops
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			return isManifestWithOps(o.Name)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectNew.(*corev1.Secret)
			return isManifestWithOps(o.Name)
		},
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

func reconcilesForSecret(ctx context.Context, mgr manager.Manager, secret corev1.Secret) []reconcile.Request {
	reconciles := []reconcile.Request{}
	var err error

	instances := &bdv1.BOSHDeploymentList{}
	err = mgr.GetClient().List(ctx, &client.ListOptions{}, instances)
	if err != nil || len(instances.Items) < 1 {
		return reconciles
	}

	instanceName := strings.TrimSuffix(secret.Name, "."+names.DeploymentSecretTypeManifestWithOps.String())
	for _, instance := range instances.Items {
		if instance.Name == instanceName {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      instance.GetName(),
					Namespace: instance.GetNamespace(),
				},
			})
		}
	}

	return reconciles
}

func isManifestWithOps(name string) bool {
	if strings.HasSuffix(name, names.DeploymentSecretTypeManifestWithOps.String()) {
		return true
	}

	return false
}
