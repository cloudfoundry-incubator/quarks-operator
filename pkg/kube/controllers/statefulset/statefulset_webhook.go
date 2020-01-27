package statefulset

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"go.uber.org/zap"

	"k8s.io/api/admission/v1beta1"
	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	wh "code.cloudfoundry.org/cf-operator/pkg/kube/util/webhook"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// Mutator changes statefulset definitions
type Mutator struct {
	log     *zap.SugaredLogger
	config  *config.Config
	decoder *admission.Decoder
}

// Implement admission.Handler so the controller can handle admission request.
var _ admission.Handler = &Mutator{}

// NewMutator returns a new reconcile.Reconciler
func NewMutator(log *zap.SugaredLogger, config *config.Config) admission.Handler {
	mutatorLog := log.Named("statefulset-rollout-mutator")
	mutatorLog.Info("Creating a StatefulSet rollout mutator")

	return &Mutator{
		log:    mutatorLog,
		config: config,
	}
}

func isControlledRolloutStatefulSet(statefulset *appsv1.StatefulSet) bool {
	enabled, ok := statefulset.GetAnnotations()[AnnotationCanaryRolloutEnabled]
	return ok && enabled == "true"
}

// Handle set the partion for StatefulSets
func (m *Mutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	statefulset := &appsv1.StatefulSet{}
	oldStatefulset := &appsv1.StatefulSet{}

	err := m.decoder.Decode(req, statefulset)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if isControlledRolloutStatefulSet(statefulset) {
		if req.Operation == v1beta1.Create {
			ConfigureStatefulSetForInitialRollout(statefulset)
		} else {
			err = m.decoder.DecodeRaw(req.OldObject, oldStatefulset)
			if err != nil {
				return admission.Errored(http.StatusBadRequest, err)
			}

			m.log.Debug("Mutator handler ran for statefulset ", statefulset.Name)

			if !reflect.DeepEqual(statefulset.Spec.Template, oldStatefulset.Spec.Template) {
				m.log.Debug("StatefulSet has changed ", statefulset.Name)
				ConfigureStatefulSetForRollout(statefulset)
			}
		}
	}

	marshaledStatefulset, err := json.Marshal(statefulset)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledStatefulset)
}

// NewStatefulSetRolloutMutator creates a statefulset mutator for setting the partion
func NewStatefulSetRolloutMutator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log.Info("Setting up mutator for statefulsets")

	mutator := NewMutator(log, config)

	globalScopeType := admissionregistration.ScopeType("*")
	return &wh.OperatorWebhook{
		FailurePolicy: admissionregistration.Fail,
		Rules: []admissionregistration.RuleWithOperations{
			{
				Rule: admissionregistration.Rule{
					APIGroups:   []string{"apps"},
					APIVersions: []string{"v1"},
					Resources:   []string{"statefulsets"},
					Scope:       &globalScopeType,
				},
				Operations: []admissionregistration.OperationType{
					"CREATE",
					"UPDATE",
				},
			},
		},
		Path: "/mutate-statefulsets",
		Name: "mutate-statefulsets." + names.GroupName,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"cf-operator-ns": config.Namespace,
			},
		},
		Webhook: &admission.Webhook{
			Handler: mutator,
		},
	}
}

// Validator implements inject.Decoder.
// A decoder will be automatically injected.
var _ admission.DecoderInjector = &Mutator{}

// InjectDecoder injects the decoder.
func (m *Mutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
