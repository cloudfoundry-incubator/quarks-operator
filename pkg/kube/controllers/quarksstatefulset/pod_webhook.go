package quarksstatefulset

import (
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/monitorednamespace"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/logger"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	wh "code.cloudfoundry.org/quarks-utils/pkg/webhook"
)

// NewQuarksStatefulSetPodMutator creates a quarksStatefulSet pod mutator for managing volumes
func NewQuarksStatefulSetPodMutator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log = logger.Unskip(log, "quarks-statefulset-pod-mutator")
	log.Info("Setting up mutator for quarksStatefulSet pods")

	quarksStatefulSetMutator := NewPodMutator(log, config)

	scope := admissionregistration.NamespacedScope
	return &wh.OperatorWebhook{
		FailurePolicy: admissionregistration.Fail,
		Rules: []admissionregistration.RuleWithOperations{
			{
				Rule: admissionregistration.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
					Scope:       &scope,
				},
				Operations: []admissionregistration.OperationType{
					"CREATE",
					"UPDATE",
				},
			},
		},
		Path: "/mutate-pods",
		Name: "mutate-pods." + names.GroupName,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				monitorednamespace.LabelNamespace: config.MonitoredID,
			},
		},
		Webhook: &admission.Webhook{
			Handler: quarksStatefulSetMutator,
		},
	}
}
