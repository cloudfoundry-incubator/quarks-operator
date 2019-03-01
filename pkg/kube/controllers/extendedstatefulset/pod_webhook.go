package extendedstatefulset

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
)

// AddPod creates a new hook for working with Pods and adds it to the Manager
func AddPod(log *zap.SugaredLogger, ctrConfig *context.Config, mgr manager.Manager, hookServer *webhook.Server) (*admission.Webhook, error) {
	log.Info("Creating the ExtendedStatefulSet Pod controller")

	log.Info("Setting up pod webhooks")

	podMutator := NewPodMutator(log, ctrConfig, mgr, controllerutil.SetControllerReference)

	mutatingWebhook, err := builder.NewWebhookBuilder().
		Path("/mutate-pods").
		Mutating().
		ForType(&corev1.Pod{}).
		Handlers(podMutator).
		WithManager(mgr).
		FailurePolicy(admissionregistrationv1beta1.Fail).
		Build()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't build a new webhook")
	}

	err = hookServer.Register(mutatingWebhook)
	if err != nil {
		return nil, errors.Wrap(err, "unable to register the hook with the admission server")
	}

	return mutatingWebhook, nil
}
