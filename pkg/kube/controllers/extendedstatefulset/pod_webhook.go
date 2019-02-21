package extendedstatefulset

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllersconfig"
)

// AddPod creates a new hook for working with Pods and adds it to the Manager
func AddPod(log *zap.SugaredLogger, ctrConfig *controllersconfig.ControllersConfig, mgr manager.Manager, hookServer *webhook.Server) error {
	log.Info("Creating the ExtendedStatefulSet Pod controller")

	log.Info("Setting up pod webhooks")

	podMutator := NewPodMutator(log, ctrConfig, mgr, controllerutil.SetControllerReference)

	mutatingWebhook, err := builder.NewWebhookBuilder().
		Path("/mutate-pods").
		Mutating().
		ForType(&corev1.Pod{}).
		Handlers(podMutator).
		WithManager(mgr).
		NamespaceSelector(&metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{metav1.LabelSelectorRequirement{
			Key:      "name",
			Operator: metav1.LabelSelectorOpIn,
			Values:   []string{ctrConfig.Namespace},
		}}}).
		Build()
	if err != nil {
		return errors.Wrap(err, "couldn't build a new webhook")
	}

	err = hookServer.Register(mutatingWebhook)
	if err != nil {
		return errors.Wrap(err, "unable to register the hook with the admission server")
	}

	return nil
}

// isStatefulSetPod matches our job pods
func isStatefulSetPod(labels map[string]string) bool {
	if _, exists := labels["statefulset.kubernetes.io/pod-name"]; exists {
		return true
	}
	return false
}
