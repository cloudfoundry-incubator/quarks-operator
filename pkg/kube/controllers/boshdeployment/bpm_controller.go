package boshdeployment

import (
	"context"
	"strings"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// AddBPM creates a new BPM Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddBPM(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "bpm-reconciler", mgr.GetRecorder("bpm-recorder"))
	r := NewBPMReconciler(ctx, config, mgr, bdm.NewResolver(mgr.GetClient(), func() bdm.Interpolator { return bdm.NewInterpolator() }), controllerutil.SetControllerReference)

	// Create a new controller
	c, err := controller.New("bpm-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch versioned secret referenced by BOSHDeployment instance
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			return isVersionedSecret(o) && isIGResolvedManifest(o.Name)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectNew.(*corev1.Secret)
			return isVersionedSecret(o) && isIGResolvedManifest(o.Name)
		},
	}

	mapSecrets := handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
		secret := a.Object.(*corev1.Secret)
		return reconcilesForVersionedSecret(ctx, mgr, *secret)
	})

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: mapSecrets}, p)
	if err != nil {
		return err
	}

	return nil
}

func isVersionedSecret(secret *corev1.Secret) bool {
	secretLabels := secret.GetLabels()
	if secretLabels == nil {
		return false
	}

	secretKind, ok := secretLabels[versionedsecretstore.LabelSecretKind]
	if !ok {
		return false
	}
	if secretKind != versionedsecretstore.VersionSecretKind {
		return false
	}

	return true
}

func isIGResolvedManifest(name string) bool {
	if strings.Contains(name, names.DeploymentSecretTypeInstanceGroupResolvedProperties.String()) {
		return true
	}

	return false
}

func reconcilesForVersionedSecret(ctx context.Context, mgr manager.Manager, secret corev1.Secret) []reconcile.Request {
	reconciles := []reconcile.Request{}

	// add requests for the BOSHDeployments referencing the versioned secret
	secretLabels := secret.GetLabels()
	if secretLabels == nil {
		return reconciles
	}

	deploymentName, ok := secretLabels[bdv1.LabelDeploymentName]
	if !ok {
		return reconciles
	}

	deployments := &bdv1.BOSHDeploymentList{}
	err := mgr.GetClient().List(ctx, &client.ListOptions{}, deployments)
	if err != nil || len(deployments.Items) < 1 {
		return reconciles
	}

	for _, deployment := range deployments.Items {
		if deployment.GetName() == deploymentName {
			reconciles = append(reconciles, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      deployment.GetName(),
					Namespace: deployment.GetNamespace(),
				},
			})
		}
	}

	return reconciles
}
