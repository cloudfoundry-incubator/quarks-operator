package extendedstatefulset

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
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

	mapSecrets := handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
		secret := a.Object.(*corev1.Secret)
		return reconcilesForSecret(ctx, mgr, *secret)
	})

	// Watch Secrets owned by resource ExtendedStatefulSet or referenced by resource ExtendedStatefulSet
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: mapSecrets})
	if err != nil {
		return err
	}

	return nil
}

func reconcilesForSecret(ctx context.Context, mgr manager.Manager, secret corev1.Secret) []reconcile.Request {
	reconciles := []reconcile.Request{}

	// add requests for the ExtendedStatefulSet owning the secret
	exStsKind := essv1.ExtendedStatefulSet{}.Kind
	for _, ref := range secret.GetOwnerReferences() {
		refGV, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil
		}

		if ref.Kind == exStsKind && refGV.Group == apis.GroupName {
			reconciles = append(reconciles, reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: secret.GetNamespace(),
				Name:      ref.Name,
			}})
		}
	}

	// add requests for the ExtendedStatefulSet referencing the versioned secret
	secretLabels := secret.GetLabels()
	if secretLabels == nil {
		return reconciles
	}

	secretKind, ok := secretLabels[versionedsecretstore.LabelSecretKind]
	if !ok {
		return reconciles
	}
	if secretKind != versionedsecretstore.VersionSecretKind {
		return reconciles
	}

	referencedSecretName := names.GetPrefixFromVersionedSecretName(secret.GetName())
	if referencedSecretName == "" {
		return reconciles
	}

	exStatefulSets := &essv1.ExtendedStatefulSetList{}
	err := mgr.GetClient().List(ctx, &client.ListOptions{}, exStatefulSets)
	if err != nil || len(exStatefulSets.Items) < 1 {
		return reconciles
	}

	for _, exStatefulSet := range exStatefulSets.Items {
		_, referencedSecrets := owner.GetConfigNamesFromSpec(exStatefulSet.Spec.Template.Spec.Template.Spec)
		if _, ok := referencedSecrets[referencedSecretName]; ok {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      exStatefulSet.GetName(),
					Namespace: exStatefulSet.GetNamespace(),
				},
			})
		}
	}

	return reconciles
}
