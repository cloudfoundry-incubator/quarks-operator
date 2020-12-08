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

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/withops"
	qsv1a1 "code.cloudfoundry.org/quarks-secret/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/monitorednamespace"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/ratelimiter"
	"code.cloudfoundry.org/quarks-utils/pkg/skip"
)

// AddWithOps creates a new WithOps controller to watch for
// withops secret and starts the rendering, which will
// finally produce the desired manifest secret.
func AddWithOps(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "withops-reconciler", mgr.GetEventRecorderFor("withops-recorder"))
	r := NewWithOpsReconciler(
		ctx, config, mgr,
		withops.NewResolver(
			mgr.GetClient(),
			func() withops.Interpolator { return withops.NewInterpolator() },
		),
		controllerutil.SetControllerReference,
		func(m bdm.Manifest) (boshdns.DomainNameService, error) { return boshdns.New(m) },
	)

	// Create a new controller
	c, err := controller.New("withops-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxBoshDeploymentWorkers,
		RateLimiter:             ratelimiter.New(),
	})
	if err != nil {
		return errors.Wrap(err, "Adding withops controller to manager failed.")
	}

	nsPred := monitorednamespace.NewNSPredicate(ctx, mgr.GetClient(), config.MonitoredID)

	// Watch the withops secret
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			shouldProcessEvent := isWithOpsSecret(o)
			if shouldProcessEvent {
				ctxlog.NewPredicateEvent(o).Debug(
					ctx, e.Meta, names.Secret,
					fmt.Sprintf("Create predicate passed for '%s/%s', secret with label %s, value %s",
						e.Meta.GetNamespace(), e.Meta.GetName(), bdv1.LabelDeploymentSecretType, o.GetLabels()[bdv1.LabelDeploymentSecretType]),
				)
			}

			return shouldProcessEvent
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret := e.ObjectOld.(*corev1.Secret)
			newSecret := e.ObjectNew.(*corev1.Secret)

			shouldProcessEvent := isWithOpsSecret(newSecret)
			if shouldProcessEvent && !reflect.DeepEqual(oldSecret.Data, newSecret.Data) {
				ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
					ctx, e.MetaNew, names.Secret,
					fmt.Sprintf("Update predicate passed for '%s/%s', existing secret with label %s, value %s",
						newSecret.GetNamespace(), newSecret.GetName(), bdv1.LabelDeploymentSecretType, newSecret.GetLabels()[bdv1.LabelDeploymentSecretType]),
				)
				return true
			}
			return false
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching secret failed in withops controller.")
	}

	// Watch explicit secrets
	p = predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectOld.(*corev1.Secret)
			n := e.ObjectNew.(*corev1.Secret)

			ok := isDeploymentExplicitSecret(n)

			return !reflect.DeepEqual(o.Data, n.Data) && ok
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			s := a.Object.(*corev1.Secret)

			if skip.Reconciles(ctx, mgr.GetClient(), s) {
				return []reconcile.Request{}
			}
			result := []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      bdv1.DeploymentSecretTypeManifestWithOps.String(),
						Namespace: s.Namespace,
					},
				},
			}
			ctxlog.NewPredicateEvent(a.Object).Debug(
				ctx, a.Meta, names.Secret,
				fmt.Sprintf("Update predicate passed for existing secret '%s/%s'", s.GetNamespace(), s.GetName()),
			)

			return result
		}),
	}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in withops controller.")
	}

	return nil
}

func isWithOpsSecret(secret *corev1.Secret) bool {
	secretLabels := secret.GetLabels()
	deploymentSecretType, ok := secretLabels[bdv1.LabelDeploymentSecretType]
	if !ok {
		return false
	}
	if deploymentSecretType != bdv1.DeploymentSecretTypeManifestWithOps.String() {
		return false
	}

	return true
}

func isDeploymentExplicitSecret(secret *corev1.Secret) bool {
	secretLabels := secret.GetLabels()
	if !bdv1.HasDeploymentName(secretLabels) {
		return false
	}
	value, ok := secretLabels[qsv1a1.LabelKind]
	if !ok {
		return false
	}
	if value != "generated" {
		return false
	}

	return true
}
