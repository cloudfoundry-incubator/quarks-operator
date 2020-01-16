package quarkslink

import (
	"context"
	"fmt"
	"regexp"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/reference"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

const name = "quarks-link-restart"

var secretNameRegex = regexp.MustCompile("^link-[a-z0-9-]*$")

// AddRestart creates a new controller to restart statefulsets and deployments
// if one of their pods has changed entanglement information
func AddRestart(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, name+"-reconciler", mgr.GetEventRecorderFor(name+"-recorder"))
	r := NewRestartReconciler(ctx, config, mgr)

	c, err := controller.New(name+"-controller", mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return errors.Wrap(err, "Adding restart controller to manager failed.")
	}

	// watch entanglement secrets, trigger if one changes which is used by a pod
	p := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			// is it a modification to an entanglement secret?
			nameMatch := secretNameRegex.MatchString(e.MetaNew.GetName())
			if !nameMatch {
				return false
			}

			labels := e.MetaNew.GetLabels()
			if _, found := labels[manifest.LabelDeploymentName]; !found {
				return false
			}

			ctxlog.NewPredicateEvent(e.ObjectNew).Debug(
				ctx, e.MetaNew, "corev1.Secret",
				fmt.Sprintf("Update predicate passed for '%s'", e.MetaNew.GetName()),
			)
			return true
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			secret := a.Object.(*corev1.Secret)

			c := mgr.GetClient()

			if reference.SkipReconciles(ctx, c, secret) {
				return []reconcile.Request{}
			}

			reconciles := []reconcile.Request{}
			namespace := secret.GetNamespace()

			list := &corev1.PodList{}
			c.List(ctx, list, client.InNamespace(namespace))

			for _, pod := range list.Items {
				if !validEntanglement(pod.GetAnnotations()) {
					continue
				}

				e := newEntanglement(pod.GetAnnotations())
				if _, ok := e.find(*secret); ok {
					reconciles = append(reconciles, reconcile.Request{
						NamespacedName: types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace},
					})
				}
			}

			for _, reconcile := range reconciles {
				ctxlog.NewMappingEvent(a.Object).Debug(ctx, reconcile, "QuarksLinkRestart", a.Meta.GetName(), "secret")
			}

			return reconciles
		}),
	}, p)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in %s controller.", name)
	}

	return nil
}
