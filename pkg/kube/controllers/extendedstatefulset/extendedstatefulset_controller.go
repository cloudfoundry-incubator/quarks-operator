package extendedstatefulset

import (
	"context"

	"github.com/pkg/errors"

	appsv1beta2client "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// AddExtendedStatefulSet creates a new ExtendedStatefulSet controller and adds it to the Manager
func AddExtendedStatefulSet(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "ext-statefulset-reconciler", mgr.GetEventRecorderFor("ext-statefulset-recorder"))
	store := vss.NewVersionedSecretStore(mgr.GetClient())
	r := NewReconciler(ctx, config, mgr, controllerutil.SetControllerReference, store)

	// Create a new controller
	c, err := controller.New("ext-statefulset-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxExtendedStatefulSetWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding Extendedstatefulset controller to manager failed.")
	}

	client, err := appsv1beta2client.NewForConfig(mgr.GetConfig())
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}

	// Watch for changes to primary resource ExtendedStatefulSet
	// Trigger when
	// - create event of extendedStatefulSet which have no children resources
	// - update event of extendedStatefulSet
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*estsv1.ExtendedStatefulSet)
			sts, err := listStatefulSetsFromAPIClient(ctx, client, o)
			if err != nil {
				ctxlog.Errorf(ctx, "Failed to list StatefulSets owned by ExtendedStatefulSet '%s': %s", o.Name, err)
			}

			return len(sts) == 0
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
	}
	err = c.Watch(&source.Kind{Type: &estsv1.ExtendedStatefulSet{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	return nil
}
