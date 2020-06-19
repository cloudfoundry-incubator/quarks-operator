package boshdeployment

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/qjobs"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/monitorednamespace"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/reference"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/withops"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// AddDeployment creates a new BOSHDeployment controller to watch for
// BOSHDeployment manifest custom resources and start the rendering, which will
// finally produce the "desired manifest", the instance group manifests and the BPM configs.
func AddDeployment(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "boshdeployment-reconciler", mgr.GetEventRecorderFor("boshdeployment-recorder"))
	r := NewDeploymentReconciler(
		ctx, config, mgr,
		withops.NewResolver(
			mgr.GetClient(),
			func() withops.Interpolator { return withops.NewInterpolator() },
		),
		qjobs.NewJobFactory(),
		converter.NewVariablesConverter(),
		controllerutil.SetControllerReference,
	)

	// Create a new controller
	c, err := controller.New("boshdeployment-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxBoshDeploymentWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding Bosh deployment controller to manager failed.")
	}

	nsPred := monitorednamespace.NewNSPredicate(ctx, mgr.GetClient(), config.MonitoredID)

	// Watch for changes to primary resource BOSHDeployment
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			ctxlog.NewPredicateEvent(e.Object).Debug(
				ctx, e.Meta, "bdv1.BOSHDeployment",
				fmt.Sprintf("Create predicate passed for '%s/%s'", e.Meta.GetNamespace(), e.Meta.GetName()),
			)
			return true
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*bdv1.BOSHDeployment)
			n := e.ObjectNew.(*bdv1.BOSHDeployment)
			if !reflect.DeepEqual(o.Spec, n.Spec) {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, "bdv1.BOSHDeployment",
					fmt.Sprintf("Update predicate passed for '%s/%s'", e.MetaNew.GetNamespace(), e.MetaNew.GetName()),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &bdv1.BOSHDeployment{}}, &handler.EnqueueRequestForObject{}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching bosh deployment failed in bosh deployment controller.")
	}

	// Watch ConfigMaps referenced by the BOSHDeployment
	p = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldConfigMap := e.ObjectOld.(*corev1.ConfigMap)
			newConfigMap := e.ObjectNew.(*corev1.ConfigMap)

			return !reflect.DeepEqual(oldConfigMap.Data, newConfigMap.Data)
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			config := a.Object.(*corev1.ConfigMap)

			if reference.SkipReconciles(ctx, mgr.GetClient(), config) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, config, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for config '%s/%s': %v", config.Namespace, config.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "BOSHDeployment", a.Meta.GetName(), bdv1.ConfigMapReference)
			}

			return reconciles
		}),
	}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching configmaps failed in bosh deployment controller.")
	}

	// Watch Secrets referenced by the BOSHDeployment
	p = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			secret := e.Object.(*corev1.Secret)
			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, secret, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s/%s': %v", secret.Namespace, secret.Name, err)
			}

			// The Secret should reference at least one BOSHDeployment in order for us to consider it
			return len(reconciles) > 1
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret := e.ObjectOld.(*corev1.Secret)
			newSecret := e.ObjectNew.(*corev1.Secret)

			return !reflect.DeepEqual(oldSecret.Data, newSecret.Data)
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			secret := a.Object.(*corev1.Secret)

			if reference.SkipReconciles(ctx, mgr.GetClient(), secret) {
				return []reconcile.Request{}
			}

			reconciles, err := reference.GetReconciles(ctx, mgr.GetClient(), reference.ReconcileForBOSHDeployment, secret, false)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to calculate reconciles for secret '%s/%s': %v", secret.Namespace, secret.Name, err)
			}

			for _, reconciliation := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciliation, "BOSHDeployment", a.Meta.GetName(), bdv1.SecretReference)
			}

			return reconciles
		}),
	}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in bosh deployment controller.")

	}

	// Watch Services that route (select) pods that are external link providers
	p = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			service := e.Object.(*corev1.Service)

			return isLinkProviderService(service)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			newService := e.ObjectNew.(*corev1.Service)

			return isLinkProviderService(newService)
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			// Get one request from one service at most
			reconciles := make([]reconcile.Request, 1)

			svc := a.Object.(*corev1.Service)
			if provider, ok := svc.GetAnnotations()[bdv1.AnnotationLinkProviderService]; ok {
				reconciles[0] = reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: svc.Namespace,
						Name:      provider,
					},
				}
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconciles[0], "BOSHDeployment", a.Meta.GetName(), "ServiceOfLinkProvider")
			}

			return reconciles
		}),
	}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "watching services failed in bosh deployment controller.")

	}

	return nil
}
