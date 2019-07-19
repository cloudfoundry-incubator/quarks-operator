package boshdeployment

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

// AddGeneratedVariable creates a new generated variable Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddGeneratedVariable(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "generated-variable-reconciler", mgr.GetRecorder("generated-variable-recorder"))
	r := NewGeneratedVariableReconciler(
		ctx, config, mgr,
		controllerutil.SetControllerReference,
		converter.NewKubeConverter(config.Namespace),
	)

	// Create a new controller
	c, err := controller.New("generated-variable-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch Secrets which contain manifest with ops
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			shouldProcessEvent := isManifestWithOps(o.Name)

			if shouldProcessEvent {
				ctxlog.NewPredicateEvent(o).Debug(
					ctx, e.Meta, bdv1.SecretType,
					fmt.Sprintf("Create predicate passed for %s, existing secret with the %s suffix",
						e.Meta.GetName(), names.DeploymentSecretTypeManifestWithOps.String()),
				)
			}

			return shouldProcessEvent
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldSecret := e.ObjectOld.(*corev1.Secret)
			newSecret := e.ObjectNew.(*corev1.Secret)
			shouldProcessEvent := isManifestWithOps(newSecret.Name) && !reflect.DeepEqual(oldSecret.Data, newSecret.Data)

			if shouldProcessEvent {
				ctxlog.NewPredicateEvent(newSecret).Debug(
					ctx, e.MetaNew, bdv1.SecretType,
					fmt.Sprintf("Update predicate passed for %s, existing secret with the %s suffix has been updated",
						e.MetaNew.GetName(), names.DeploymentSecretTypeManifestWithOps.String()),
				)
			}

			return shouldProcessEvent
		},
	}

	// This is a manifest with ops files secret that has changed.
	// We can reconcile this as-is, no need to find the corresponding BOSHDeployment.
	// All we have to do is create secrets for explicit variables.
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	return nil
}

func isManifestWithOps(name string) bool {
	// TODO: replace this with an annotation
	return strings.HasSuffix(name, names.DeploymentSecretTypeManifestWithOps.String())
}
