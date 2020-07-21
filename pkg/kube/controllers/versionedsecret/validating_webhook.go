package versionedsecret

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	"go.uber.org/zap"

	"k8s.io/api/admission/v1beta1"
	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/monitorednamespace"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/logger"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
	wh "code.cloudfoundry.org/quarks-utils/pkg/webhook"
)

// NewSecretValidator creates a validating hook to deny updates to versioned secrets and adds it to the manager.
func NewSecretValidator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log = logger.Unskip(log, "secret-validator")
	log.Info("Setting up validator for versioned secrets")

	secretValidator := NewValidationHandler(log)

	globalScopeType := admissionregistration.NamespacedScope
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
				monitorednamespace.LabelNamespace: config.MonitoredID,
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
	validationLog := log.Named("secret-validator").Desugar().WithOptions(zap.AddCallerSkip(-1)).Sugar()
	validationLog.Info("Creating a validator for Secret")

	return &ValidationHandler{
		log: validationLog,
	}
}

// Handle denies changes to all versioned secrets as they are immutable
func (v *ValidationHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	secret := &corev1.Secret{}

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

	oldSecret := &corev1.Secret{}
	err = v.decoder.DecodeRaw(req.OldObject, oldSecret)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Checking if the secret is a versioned secret
	ok := vss.IsVersionedSecret(*oldSecret)
	if ok {
		v.log.Info("found versioned secret")

		dataChanged := !reflect.DeepEqual(secret.Data, oldSecret.Data)

		if dataChanged {
			v.log.Infof("Denying update to versioned secret '%s/%s' data as it is immutable.", secret.Namespace, secret.Name)
			return admission.Response{
				AdmissionResponse: v1beta1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: fmt.Sprintf("Denying update to versioned secret '%s/%s' as it is immutable.", secret.Namespace, secret.Name),
					},
				},
			}
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
