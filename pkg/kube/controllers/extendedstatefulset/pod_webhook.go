package extendedstatefulset

import (
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/quarks-utils/pkg/config"
	wh "code.cloudfoundry.org/cf-operator/pkg/kube/util/webhook"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewExtendedStatefulsetPodMutator creates a pod mutator for managing volumes
func NewExtendedStatefulsetPodMutator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log.Info("Setting up mutator for pods")

	extendedStatefulSetMutator := NewPodMutator(log, config)

	globalScopeType := admissionregistrationv1beta1.ScopeType("*")
	return &wh.OperatorWebhook{
		FailurePolicy: admissionregistrationv1beta1.Fail,
		Rules: []admissionregistrationv1beta1.RuleWithOperations{
			{
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
					Scope:       &globalScopeType,
				},
				Operations: []admissionregistrationv1beta1.OperationType{
					"CREATE",
					"UPDATE",
				},
			},
		},
		Path: "/mutate-pods",
		Name: "mutate-pods.fissile.cloudfoundry.org",
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"cf-operator-ns": config.Namespace,
			},
		},
		Webhook: &admission.Webhook{
			Handler: extendedStatefulSetMutator,
		},
	}
}
