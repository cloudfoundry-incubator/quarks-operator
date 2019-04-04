package boshdeployment

import (
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	manifestStore "code.cloudfoundry.org/cf-operator/pkg/kube/util/store/manifest"
)

// AddManifest creates a new BOSHDeployment controller to start when desired manifest creation
func AddManifest(log *zap.SugaredLogger, config *context.Config, mgr manager.Manager) error {
	r := NewManifestReconciler(log, config, mgr)
	c, err := controller.New("bosh-deployment-manifest-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch new Secret which is manifest secret
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret := e.Object.(*corev1.Secret)
			// Only enqueuing manifest secret creation
			if kind, ok := secret.GetLabels()[manifestStore.LabelKind]; ok {
				return kind == "manifest"
			}
			return false
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}

	// pick up new Secret which are referenced by an boshDeployment instance
	ctx, _ := context.NewBackgroundContextWithTimeout(config.CtxType, config.CtxTimeOut)
	mapSecrets := handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
		secret := a.Object.(*corev1.Secret)
		return reconcilesForSecret(ctx, mgr, log, *secret)
	})

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: mapSecrets}, p)
	if err != nil {
		return err
	}

	return err
}

func reconcilesForSecret(ctx context.Context, mgr manager.Manager, log *zap.SugaredLogger, secret corev1.Secret) []reconcile.Request {
	reconciles := []reconcile.Request{}
	instances := &bdv1.BOSHDeploymentList{}
	err := mgr.GetClient().List(ctx, &client.ListOptions{}, instances)
	if err != nil || len(instances.Items) < 1 {
		return reconciles
	}

	secretLabels := labels.Set(secret.GetLabels())
	deploymentName := secretLabels.Get(bdv1.LabelDeploymentName)

	for _, instance := range instances.Items {
		if instance.GetName() == deploymentName {
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
