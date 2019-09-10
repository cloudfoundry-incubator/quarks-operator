package boshdeployment

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter/factory"
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
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

// AddGeneratedVariable creates a new generated variable controller to watch for the intermidiate "with-ops" manifest and
// reconcile it into one ExtendedSecret for each explicit variable.
func AddGeneratedVariable(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "generated-variable-reconciler", mgr.GetEventRecorderFor("generated-variable-recorder"))
	r := NewGeneratedVariableReconciler(
		ctx, config, mgr,
		controllerutil.SetControllerReference,
		converter.NewKubeConverter(
			config.Namespace,
			factory.NewVolumeFactory(),
			func(manifestName string, instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider converter.ReleaseImageProvider, bpmConfigs bpm.Configs) converter.ContainerFactory {
				return factory.NewContainerFactory(manifestName, instanceGroupName, version, disableLogSidecar, releaseImageProvider, bpmConfigs)
			}),
	)

	// Create a new controller
	c, err := controller.New("generated-variable-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxBoshDeploymentWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding generated variable controller to manager failed.")
	}

	// Watch Secrets which contain manifest with ops
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			shouldProcessEvent := isManifestWithOps(o.Name)

			if shouldProcessEvent {
				ctxlog.NewPredicateEvent(o).Debug(
					ctx, e.Meta, bdv1.SecretReference,
					fmt.Sprintf("Create predicate passed for %s, existing secret with the %s suffix",
						e.Meta.GetName(), names.DeploymentSecretTypeManifestWithOps.String()),
				)
			}

			return shouldProcessEvent
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}

	// This is a manifest with ops files secret that has changed.
	// We can reconcile this as-is, no need to find the corresponding BOSHDeployment.
	// All we have to do is create secrets for explicit variables.
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return errors.Wrap(err, "Watching secrets in generated variable controller.")
	}

	return nil
}

func isManifestWithOps(name string) bool {
	// TODO: replace this with an annotation
	return strings.HasSuffix(name, names.DeploymentSecretTypeManifestWithOps.String())
}
