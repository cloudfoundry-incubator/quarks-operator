package extendedstatefulset

import (
	"context"

	"k8s.io/api/apps/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// AddVersionCleanup creates a new version cleanup controller and adds it to the Manager
func AddVersionCleanup(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "version-cleanup-reconciler", mgr.GetRecorder("version-cleanup-recorder"))
	r := NewVersionCleanupReconciler(ctx, config, mgr)

	// Create a new controller
	c, err := controller.New("version-cleanup-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch new version StatefulSets owned by the ExtendedStatefulSet
	// Trigger when at least one pod of new versions is running
	statefulSetPredicates := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			newStatefulSet := e.Object.(*v1beta2.StatefulSet)
			return newStatefulSet.Status.ReadyReplicas > 0
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			newStatefulSet := e.ObjectNew.(*v1beta2.StatefulSet)

			return newStatefulSet.Status.ReadyReplicas > 0
		},
	}
	err = c.Watch(&source.Kind{Type: &v1beta2.StatefulSet{}}, &handler.EnqueueRequestForOwner{
		IsController: false,
		OwnerType:    &estsv1.ExtendedStatefulSet{},
	}, statefulSetPredicates)
	if err != nil {
		return err
	}

	return nil
}
