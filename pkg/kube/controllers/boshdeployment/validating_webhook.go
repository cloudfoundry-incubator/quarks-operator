package boshdeployment

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktype "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

const (
	admissionWebhookName = "validate-boshdeployment.fissile.cloudfoundry.org"
	admissionWebHookPath = "/validate-boshdeployment"
)

// AddBOSHDeploymentValidator creates a validating hook for BOSHDeployment and adds it to the Manager
func AddBOSHDeploymentValidator(log *zap.SugaredLogger, config *config.Config, mgr manager.Manager) (webhook.Webhook, error) {
	log.Info("Setting up validator for BOSHDeployment")

	boshDeploymentValidator := NewValidator(log, config)

	validatingWebhook, err := builder.NewWebhookBuilder().
		Name(admissionWebhookName).
		Path(admissionWebHookPath).
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
	log          *zap.SugaredLogger
	config       *config.Config
	client       client.Client
	decoder      types.Decoder
	pollTimeout  time.Duration
	pollInterval time.Duration
}

// NewValidator returns a new BOSHDeploymentValidator
func NewValidator(log *zap.SugaredLogger, config *config.Config) admission.Handler {
	validationLog := log.Named("boshdeployment-validator")
	validationLog.Info("Creating a validator for BOSHDeployment")

	return &Validator{
		log:          validationLog,
		config:       config,
		pollTimeout:  5 * time.Second,
		pollInterval: 500 * time.Millisecond,
	}
}

// OpsResourceExist verify if a resource exist in the namespace,
// it will check it´s existance during 5 seconds,
// otherwise it will timeout.
func (v *Validator) OpsResourceExist(ctx context.Context, specOpsResource bdv1.Ops, ns string) (bool, string) {
	timeOut := time.After(v.pollTimeout)
	tick := time.NewTicker(v.pollInterval)
	defer tick.Stop()

	switch specOpsResource.Type {
	case "configmap":
		key := ktype.NamespacedName{Namespace: string(ns), Name: specOpsResource.Ref}
		for {
			select {
			case <-timeOut:
				return false, fmt.Sprintf("Timeout reached. Resource %s does not exist", specOpsResource.Ref)
			case <-tick.C:
				err := v.client.Get(ctx, key, &corev1.ConfigMap{})
				if err == nil {
					return true, fmt.Sprintf("configmap %s, exists", specOpsResource.Ref)
				}
			}
		}
	case "secret":
		key := ktype.NamespacedName{Namespace: string(ns), Name: specOpsResource.Ref}
		for {
			select {
			case <-timeOut:
				return false, fmt.Sprintf("Timeout reached. Resource %s does not exist", specOpsResource.Ref)
			case <-tick.C:
				err := v.client.Get(ctx, key, &corev1.Secret{})
				if err == nil {
					return true, fmt.Sprintf("secret %s, exists", specOpsResource.Ref)
				}
			}
		}
	default:
		// We only support configmaps so far
		return false, fmt.Sprintf("resource type %s, is not supported under spec.ops", specOpsResource.Type)
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
	resolver := converter.NewResolver(v.client, func() converter.Interpolator { return converter.NewInterpolator() })

	for _, opsItem := range boshDeployment.Spec.Ops {
		resourceExist, msg := v.OpsResourceExist(ctx, opsItem, boshDeployment.Namespace)
		if !resourceExist {
			return types.Response{
				Response: &v1beta1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: msg,
					},
				},
			}
		}
	}

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
