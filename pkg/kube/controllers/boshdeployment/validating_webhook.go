package boshdeployment

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktype "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	wh "code.cloudfoundry.org/cf-operator/pkg/kube/util/webhook"
)

// NewBOSHDeploymentValidator creates a validating hook for BOSHDeployment and adds it to the Manager
func NewBOSHDeploymentValidator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log.Info("Setting up validator for BOSHDeployment")

	boshDeploymentValidator := NewValidator(log, config)

	globalScopeType := admissionregistrationv1beta1.ScopeType("*")
	return &wh.OperatorWebhook{
		FailurePolicy: admissionregistrationv1beta1.Fail,
		Rules: []admissionregistrationv1beta1.RuleWithOperations{
			{
				Rule: admissionregistrationv1beta1.Rule{
					APIGroups:   []string{"fissile.cloudfoundry.org"},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"boshdeployments"},
					Scope:       &globalScopeType,
				},
				Operations: []admissionregistrationv1beta1.OperationType{
					"CREATE",
					"UPDATE",
				},
			},
		},
		Path: "/validate-boshdeployment",
		Name: "validate-boshdeployment.fissile.cloudfoundry.org",
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"cf-operator-ns": config.Namespace,
			},
		},
		Webhook: &admission.Webhook{
			Handler: boshDeploymentValidator,
		},
	}
}

// Validator s
type Validator struct {
	log          *zap.SugaredLogger
	config       *config.Config
	client       client.Client
	decoder      *admission.Decoder
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
// it will check its existence during 5 seconds,
// otherwise it will timeout.
func (v *Validator) OpsResourceExist(ctx context.Context, specOpsResource bdv1.ResourceReference, ns string) (bool, string) {
	timeOut := time.After(v.pollTimeout)
	tick := time.NewTicker(v.pollInterval)
	defer tick.Stop()

	switch specOpsResource.Type {
	case bdv1.ConfigMapReference:
		key := ktype.NamespacedName{Namespace: string(ns), Name: specOpsResource.Name}
		for {
			select {
			case <-timeOut:
				return false, fmt.Sprintf("Timeout reached. Resource %s does not exist", specOpsResource.Name)
			case <-tick.C:
				err := v.client.Get(ctx, key, &corev1.ConfigMap{})
				if err == nil {
					return true, fmt.Sprintf("configmap %s, exists", specOpsResource.Name)
				}
			}
		}
	case bdv1.SecretReference:
		key := ktype.NamespacedName{Namespace: string(ns), Name: specOpsResource.Name}
		for {
			select {
			case <-timeOut:
				return false, fmt.Sprintf("Timeout reached. Resource %s does not exist", specOpsResource.Name)
			case <-tick.C:
				err := v.client.Get(ctx, key, &corev1.Secret{})
				if err == nil {
					return true, fmt.Sprintf("secret %s, exists", specOpsResource.Name)
				}
			}
		}
	default:
		// We only support configmaps and secrets so far
		return false, fmt.Sprintf("resource type %s, is not supported under spec.ops", specOpsResource.Type)
	}
}

//Handle validates a BOSHDeployment
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	boshDeployment := &bdv1.BOSHDeployment{}
	ctx = log.NewParentContext(v.log)

	err := v.decoder.Decode(req, boshDeployment)
	if err != nil {
		return admission.Response{
			AdmissionResponse: v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: fmt.Sprintf("Failed to decode BOSHDeployment: %s", err.Error()),
				},
			},
		}
	}

	log.Debug(ctx, "Resolving manifest")
	resolver := converter.NewResolver(v.client, func() converter.Interpolator { return converter.NewInterpolator() })

	for _, opsItem := range boshDeployment.Spec.Ops {
		resourceExist, msg := v.OpsResourceExist(ctx, opsItem, boshDeployment.Namespace)
		if !resourceExist {
			return admission.Response{
				AdmissionResponse: v1beta1.AdmissionResponse{
					Allowed: false,
					Result: &metav1.Status{
						Message: msg,
					},
				},
			}
		}
	}

	_, _, err = resolver.WithOpsManifest(ctx, boshDeployment, boshDeployment.GetNamespace())
	if err != nil {
		return admission.Response{
			AdmissionResponse: v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: fmt.Sprintf("Failed to resolve manifest: %s", err.Error()),
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
var _ inject.Client = &Validator{}

// InjectClient injects the client.
func (v *Validator) InjectClient(c client.Client) error {
	v.client = c
	return nil
}

// Validator implements inject.Decoder.
// A decoder will be automatically injected.
var _ admission.DecoderInjector = &Validator{}

// InjectDecoder injects the decoder.
func (v *Validator) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}
