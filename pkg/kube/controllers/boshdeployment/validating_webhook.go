package boshdeployment

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"

	"go.uber.org/zap"
	"k8s.io/api/admission/v1beta1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	wh "code.cloudfoundry.org/cf-operator/pkg/kube/util/webhook"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
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
					APIGroups:   []string{names.GroupName},
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
		Name: "validate-boshdeployment." + names.GroupName,
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

// OpsResourcesExist verify if a resource exist in the namespace,
// it will check its existence during 5 seconds,
// otherwise it will timeout.
func (v *Validator) OpsResourcesExist(ctx context.Context, specOpsResource []bdv1.ResourceReference, ns string) (bool, string) {
	timeOut := time.After(v.pollTimeout)
	tick := time.NewTicker(v.pollInterval)
	defer tick.Stop()

	missingResources := map[string]bool{}

	for {
		configMaps := &corev1.ConfigMapList{}
		secrets := &corev1.SecretList{}

		select {
		case <-timeOut:
			missingResourcesNames := []string{}
			for k, v := range missingResources {
				if v {
					missingResourcesNames = append(missingResourcesNames, k)
				}
			}
			return false, fmt.Sprintf("Timeout reached. Resources '%s' do not exist", strings.Join(missingResourcesNames, " "))
		case <-tick.C:
			// List all configmaps
			err := v.client.List(ctx, configMaps, client.InNamespace(ns))
			if err != nil {
				return false, fmt.Sprintf("error listing configMaps in namespace '%s': %v", ns, err)
			}

			// List all secrets
			err = v.client.List(ctx, secrets, client.InNamespace(ns))
			if err != nil {
				return false, fmt.Sprintf("error listing secrets in namespace '%s': %v", ns, err)
			}
		}

		// Check to see if all references exist
		allExist := true
		for _, ref := range specOpsResource {
			resourceName := fmt.Sprintf("%s/%s", ref.Type, ref.Name)

			found := false
			switch ref.Type {
			case bdv1.ConfigMapReference:
				for _, configMap := range configMaps.Items {
					if configMap.Name == ref.Name {
						found = true
						break
					}
				}

			case bdv1.SecretReference:
				for _, secret := range secrets.Items {
					if secret.Name == ref.Name {
						found = true
						break
					}
				}
			}

			missingResources[resourceName] = !found

			if !found {
				allExist = false
			}

		}

		if allExist {
			return true, "all references exist"
		}
	}
}

//Handle validates a BOSHDeployment
func (v *Validator) Handle(_ context.Context, req admission.Request) admission.Response {
	boshDeployment := &bdv1.BOSHDeployment{}
	ctx := log.NewParentContext(v.log)

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

	log.Infof(ctx, "Verifying dependencies for deployment '%s'", boshDeployment.Name)
	resolver := converter.NewResolver(v.client, func() converter.Interpolator { return converter.NewInterpolator() })
	resourceExist, msg := v.OpsResourcesExist(ctx, boshDeployment.Spec.Ops, boshDeployment.Namespace)
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

	log.Infof(ctx, "Resolving deployment '%s'", boshDeployment.Name)
	manifest, _, err := resolver.WithOpsManifestDetailed(ctx, boshDeployment, boshDeployment.GetNamespace())
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
	err = validateCanaryWatchTime(*manifest)
	if err != nil {
		return admission.Response{
			AdmissionResponse: v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: fmt.Sprintf("Failed to validate canary_watch_time: %s", err.Error()),
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

func validateCanaryWatchTime(manifest manifest.Manifest) error {
	if manifest.Update == nil || manifest.Update.CanaryWatchTime == "" {
		return errors.New("no canary_watch_time specified")
	}
	canaryWatchTime := manifest.Update.CanaryWatchTime
	absoluteRegex := regexp.MustCompile(`^\s*(\d+)\s*$`)
	rangeRegex := regexp.MustCompile(`^\s*(\d+)\s*-\s*(\d+)\s*$`)
	if absoluteRegex.MatchString(canaryWatchTime) || rangeRegex.MatchString(canaryWatchTime) {
		return nil
	}
	return errors.New("watch time must be an integer or a range of two integers")
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
