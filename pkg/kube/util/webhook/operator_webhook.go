package webhook

import (
	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// OperatorWebhook represents an operator webhook
type OperatorWebhook struct {
	// Name is the name of the webhook
	Name string
	// Path is the path this webhook will serve.
	Path string
	// Rules maps to the Rules field in admissionregistration.Webhook
	Rules []admissionregistration.RuleWithOperations
	// FailurePolicy maps to the FailurePolicy field in admissionregistration.Webhook
	// This optional. If not set, will be defaulted to Ignore (fail-open) by the server.
	// More details: https://github.com/kubernetes/api/blob/f5c295feaba2cbc946f0bbb8b535fc5f6a0345ee/admissionregistration/v1/types.go#L144-L147
	FailurePolicy admissionregistration.FailurePolicyType
	// NamespaceSelector maps to the NamespaceSelector field in admissionregistration.Webhook
	// This optional.
	NamespaceSelector *metav1.LabelSelector
	// Handlers contains a list of handlers. Each handler may only contains the business logic for its own feature.
	// For example, feature foo and bar can be in the same webhook if all the other configurations are the same.
	// The handler will be invoked sequentially as the order in the list.
	// Note: if you are using mutating webhook with multiple handlers, it's your responsibility to
	// ensure the handlers are not generating conflicting JSON patches.
	Handler admission.Handler
	// Webhook contains the Admission webhook information that we register with the controller runtime.
	Webhook *webhook.Admission
}
