package boshdeployment

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// AddBPM creates a new BPM Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func AddBPM(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewContextWithRecorder(ctx, "bpm-reconciler", mgr.GetRecorder("bpm-recorder"))
	r := NewBPMReconciler(
		ctx, config, mgr,
		bdm.NewResolver(mgr.GetClient(), func() bdm.Interpolator { return bdm.NewInterpolator() }),
		controllerutil.SetControllerReference,
		bdm.NewKubeConverter(config.Namespace),
	)

	// Create a new controller
	c, err := controller.New("bpm-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// We have to watch the versioned secret for each Instance Group
	p := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			o := e.Object.(*corev1.Secret)
			shouldProcessEvent := isBPMInfoSecret(o)
			if shouldProcessEvent {
				ctxlog.WithPredicateEvent(o).DebugPredicate(
					ctx, e.Meta, bdv1.SecretType,
					fmt.Sprintf("Create predicate passed for %s, existing secret with label %s, value %s",
						e.Meta.GetName(), bdv1.LabelDeploymentSecretType, o.GetLabels()[bdv1.LabelDeploymentSecretType]),
				)
			}

			return shouldProcessEvent
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			o := e.ObjectNew.(*corev1.Secret)
			shouldProcessEvent := isBPMInfoSecret(o)
			if shouldProcessEvent {
				ctxlog.WithPredicateEvent(o).DebugPredicate(
					ctx, e.MetaNew, bdv1.SecretType,
					fmt.Sprintf("Update predicate passed for %s, new secret with label %s, value %s",
						e.MetaNew.GetName(), bdv1.LabelDeploymentSecretType, o.GetLabels()[bdv1.LabelDeploymentSecretType]),
				)
			}

			return shouldProcessEvent
		},
	}

	// We have to watch the BPM secret. It gives us information about how to
	// start containers for each process.
	// The BPM secret is annotated with the name of the BOSHDeployment.

	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
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
