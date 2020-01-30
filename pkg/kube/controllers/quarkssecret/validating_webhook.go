package quarkssecret

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"k8s.io/api/admission/v1beta1"
	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	wh "code.cloudfoundry.org/cf-operator/pkg/kube/util/webhook"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

// NewSecretValidator creates a validating hook to deny updates to versioned secrets and adds it to the manager.
func NewSecretValidator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log.Info("Setting up validator for Secret")

	secretValidator := NewValidationHandler(log)

	globalScopeType := admissionregistration.ScopeType("*")
	return &wh.OperatorWebhook{
		FailurePolicy: admissionregistration.Fail,
		Rules: []admissionregistration.RuleWithOperations{
			{
				Rule: admissionregistration.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"secrets"},
					Scope:       &globalScopeType,
				},
				Operations: []admissionregistration.OperationType{
					"UPDATE",
				},
			},
		},
		Path: "/validate-secret",
		Name: "validate-secret." + names.GroupName,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"cf-operator-ns": config.Namespace,
			},
		},
		Webhook: &admission.Webhook{
			Handler: secretValidator,
		},
	}
}

// ValidationHandler is a struct for secret validator object.
type ValidationHandler struct {
	log     *zap.SugaredLogger
	client  client.Client
	decoder *admission.Decoder
}

// NewValidationHandler returns a new ValidationHandler
func NewValidationHandler(log *zap.SugaredLogger) admission.Handler {
	validationLog := log.Named("secret-validator")
	validationLog.Info("Creating a validator for Secret")
	return &ValidationHandler{
		log: validationLog,
	}
}

//Handle denies changes to all versioned secrets as they are immutable
func (v *ValidationHandler) Handle(_ context.Context, req admission.Request) admission.Response {
	secret := &corev1.Secret{}
	ctx := log.NewParentContext(v.log)

	err := v.decoder.Decode(req, secret)
	if err != nil {
		return admission.Response{
			AdmissionResponse: v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: fmt.Sprintf("Failed to decode secret: %s", err.Error()),
				},
			},
		}
	}

	// Checking if the secret is a versioned secret
	ok := vss.IsVersionedSecret(*secret)
	if ok {
		log.Infof(ctx, "Denying update to versioned secret '%s' as it is immutable.", secret.Name)
		return admission.Response{
			AdmissionResponse: v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: fmt.Sprintf("Denying update to versioned secret '%s' as it is immutable.", secret.GetName()),
				},
			},
		}
	}

	return admission.Response{
		AdmissionResponse: v1beta1.AdmissionResponse{
			Allowed: true,
		},
	}
}

// Validator implements inject.Client.
// A client will be automatically injected.
var _ inject.Client = &ValidationHandler{}

// InjectClient injects the client.
func (v *ValidationHandler) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// Validator implements inject.Decoder.
// A decoder will be automatically injected.
var _ admission.DecoderInjector = &ValidationHandler{}

// InjectDecoder injects the decoder.
func (v *ValidationHandler) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
