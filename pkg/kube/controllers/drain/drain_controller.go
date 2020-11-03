package quarksdrain

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/monitorednamespace"
)

// PreDrain is the annotation required on thepod for the quarks drain feature
var PreDrain = fmt.Sprintf("%s/pre-stop-script", apis.GroupName)

const name = "quarks-drain"

// AddDrain creates a new controller to wait for containers with drain scripts
func AddDrain(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, name+"-reconciler", mgr.GetEventRecorderFor(name+"-recorder"))
	r := NewDrainReconciler(ctx, config, mgr)

	c, err := controller.New(name+"-controller", mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return errors.Wrap(err, "Adding restart controller to manager failed.")
	}

	nsPred := monitorednamespace.NewNSPredicate(ctx, mgr.GetClient(), config.MonitoredID)

	// watch secrets, trigger if one changes which is used by a pod
	p := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return false },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool { // if  doesn't  have finalizer
			return true
		},
	}
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
			pod := a.Object.(*corev1.Pod)

			if !preDrainAnnotationFunc(*pod) {
				return []reconcile.Request{}
			}

			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				}}}

		}),
	}, nsPred, p)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in Restart controller failed.")
	}

	return nil
}

func preDrainAnnotationFunc(pod corev1.Pod) bool {
	annotations := pod.GetAnnotations()
	if _, found := annotations[PreDrain]; !found {
		return false
	}
	return true
}
