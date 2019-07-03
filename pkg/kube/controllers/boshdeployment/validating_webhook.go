package boshdeployment

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// AddBOSHDeploymentValidator creates a validating hook for BOSHDeployment and adds it to the Manager
func AddBOSHDeploymentValidator(log *zap.SugaredLogger, config *config.Config, mgr manager.Manager) (webhook.Webhook, error) {
	log.Info("Setting up validator for BOSHDeployment")

	boshDeploymentValidator := NewValidator(log, config)

	validatingWebhook, err := builder.NewWebhookBuilder().
		Name("validate-boshdeployment.fissile.cloudfoundry.org").
		Path("/validate-boshdeployment").
		Validating().
		NamespaceSelector(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"cf-operator-ns": config.Namespace,
			},
		}).
		ForType(&bdv1.BOSHDeployment{}).
		Handlers(boshDeploymentValidator).
		WithManager(mgr).
		FailurePolicy(admissionregistrationv1beta1.Fail).
		Build()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't build a new validating webhook")
	}

	return validatingWebhook, nil
}

// Validator represents a validator for BOSHDeployments
type Validator struct {
	log     *zap.SugaredLogger
	config  *config.Config
	client  client.Client
	decoder types.Decoder
}

// NewValidator returns a new BOSHDeploymentValidator
func NewValidator(log *zap.SugaredLogger, config *config.Config) admission.Handler {
	validationLog := log.Named("boshdeployment-validator")
	validationLog.Info("Creating a validator for BOSHDeployment")

	return &Validator{
		log:    validationLog,
		config: config,
	}
}

// Handle validates a BOSHDeployment
func (v *Validator) Handle(ctx context.Context, req types.Request) types.Response {
	boshDeployment := &bdv1.BOSHDeployment{}

	err := v.decoder.Decode(req, boshDeployment)
	if err != nil {
		return types.Response{}
	}

	log.Debug(ctx, "Resolving manifest")
	resolver := bdm.NewResolver(v.client, func() bdm.Interpolator { return bdm.NewInterpolator() })
	_, err = resolver.WithOpsManifest(boshDeployment, boshDeployment.GetNamespace())
	if err != nil {
		return types.Response{
			Response: &v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: fmt.Sprintf("Failed to resolve manifest: %s", err.Error()),
				},
			},
		}
	}

	return types.Response{
		Response: &v1beta1.AdmissionResponse{
			Allowed: true,
		},
	}
}

// podAnnotator implements inject.Client.
// A client will be automatically injected.
var _ inject.Client = &Validator{}

// InjectClient injects the client.
func (v *Validator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// podAnnotator implements inject.Decoder.
// A decoder will be automatically injected.
var _ inject.Decoder = &Validator{}

// InjectDecoder injects the decoder.
func (v *Validator) InjectDecoder(d types.Decoder) error {
	v.decoder = d
	return nil
}
