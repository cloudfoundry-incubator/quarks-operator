package boshdeployment

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// AddBPM creates a new BPM controller to watch for BPM configs and instance
// group manifests.  It will reconcile those into k8s resources
// (QuarksStatefulSet, QuarksJob), which represent BOSH instance groups and
// BOSH errands.
func AddBPM(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "bpm-reconciler", mgr.GetEventRecorderFor("bpm-recorder"))
	desiredManifest := manifest.NewDesiredManifest()
	desiredManifest.InjectClient(mgr.GetClient())
	r := NewBPMReconciler(
		ctx, config, mgr,
		desiredManifest,
		controllerutil.SetControllerReference,
		converter.NewKubeConverter(
			config.Namespace,
			converter.NewVolumeFactory(),
			func(manifestName string, instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider converter.ReleaseImageProvider, bpmConfigs bpm.Configs) converter.ContainerFactory {
				return converter.NewContainerFactory(manifestName, instanceGroupName, version, disableLogSidecar, releaseImageProvider, bpmConfigs)
			}),
	)

	// Create a new controller
	c, err := controller.New("bpm-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: config.MaxBoshDeploymentWorkers,
	})
	if err != nil {
		return errors.Wrap(err, "Adding BPM controller to manager failed.")
	}

	// We have to watch the versioned secret for each Instance Group
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			shouldProcessEvent := isBPMInfoSecret(o)

			if shouldProcessEvent {
				if metav1.HasAnnotation(o.ObjectMeta, meltdown.AnnotationLastReconcile) {
					return false
				}

				ctxlog.NewPredicateEvent(o).Debug(
					ctx, e.Meta, names.Secret,
					fmt.Sprintf("Create predicate passed for %s, existing secret with label %s, value %s",
						e.Meta.GetName(), bdv1.LabelDeploymentSecretType, o.GetLabels()[bdv1.LabelDeploymentSecretType]),
				)
			}

			return shouldProcessEvent
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc:  func(e event.UpdateEvent) bool { return false },
	}

	// We have to watch the BPM secret. It gives us information about how to
	// start containers for each process.
	// The BPM secret is annotated with the name of the BOSHDeployment.
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return errors.Wrapf(err, "Watching secrets failed in BPM controller.")
	}

	return nil
}

func isBPMInfoSecret(secret *corev1.Secret) bool {
	ok := vss.IsVersionedSecret(*secret)
	if !ok {
		return false
	}

	secretLabels := secret.GetLabels()
	deploymentSecretType, ok := secretLabels[bdv1.LabelDeploymentSecretType]
	if !ok {
		return false
	}
	if deploymentSecretType != names.DeploymentSecretBpmInformation.String() {
		return false
	}

	return true
}
