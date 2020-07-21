package boshdeployment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"k8s.io/api/admission/v1beta1"
	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/monitorednamespace"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/withops"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/logger"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	wh "code.cloudfoundry.org/quarks-utils/pkg/webhook"
)

// NewBOSHDeploymentValidator creates a validating hook for BOSHDeployment and adds it to the Manager
func NewBOSHDeploymentValidator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log.Info("Setting up validator for BOSHDeployment")

	boshDeploymentValidator := NewValidator(log, config)

	globalScopeType := admissionregistration.NamespacedScope
	return &wh.OperatorWebhook{
		FailurePolicy: admissionregistration.Fail,
		Rules: []admissionregistration.RuleWithOperations{
			{
				Rule: admissionregistration.Rule{
					APIGroups:   []string{names.GroupName},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"boshdeployments"},
					Scope:       &globalScopeType,
				},
				Operations: []admissionregistration.OperationType{
					"CREATE",
					"UPDATE",
				},
			},
		},
		Path: "/validate-boshdeployment",
		Name: "validate-boshdeployment." + names.GroupName,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				monitorednamespace.LabelNamespace: config.MonitoredID,
			},
		},
		Webhook: &admission.Webhook{
			Handler: boshDeploymentValidator,
		},
	}
}

// Validator struct contains all fields for the deployment validator
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
	validationLog := logger.Unskip(log, "boshdeployment-validator")
	validationLog.Info("Creating a validator for BOSHDeployment")

	return &Validator{
		log:          validationLog,
		config:       config,
		pollTimeout:  5 * time.Second,
		pollInterval: 500 * time.Millisecond,
	}
}

//Handle validates a BOSHDeployment
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	boshDeployment := &bdv1.BOSHDeployment{}

	err := v.decoder.Decode(req, boshDeployment)
	if err != nil {
		denied(fmt.Sprintf("Failed to decode BOSHDeployment: %s", err.Error()))
	}

	// verify only one bdpl exists in this namespace
	bdpls := &bdv1.BOSHDeploymentList{}
	err = v.client.List(ctx, bdpls, client.InNamespace(boshDeployment.GetNamespace()))
	if err != nil {
		return denied(fmt.Sprintf("Failed to list bdpl: %s", err.Error()))
	}
	if len(bdpls.Items) > 0 {
		for _, bdpl := range bdpls.Items {
			if bdpl.Name != boshDeployment.Name {
				return denied(fmt.Sprintf("Only one deployment allowed per namespace. Namespace is used for '%s'", bdpl.Name))
			}
		}
	}

	// verify dependencies exist
	v.log.Debugf("Verifying dependencies for deployment '%s'", boshDeployment.Name)
	resourceExist, msg := v.opsResourcesExist(ctx, boshDeployment.Spec.Ops, boshDeployment.Namespace)
	if !resourceExist {
		return denied(msg)
	}

	// verify with-ops manifest
	v.log.Debugf("Resolving deployment '%s'", boshDeployment.Name)
	resolver := withops.NewResolver(
		v.client,
		func() withops.Interpolator { return withops.NewInterpolator() },
	)
	manifest, _, err := resolver.ManifestDetailed(ctx, boshDeployment, boshDeployment.GetNamespace())
	if err != nil {
		return denied(fmt.Sprintf("Failed to resolve manifest: %s", err.Error()))
	}

	err = validateUpdateBlock(manifest.Update)
	if err != nil {
		return denied(fmt.Sprintf("Failed to validate update block: %s", err.Error()))
	}

	return admission.Response{
		AdmissionResponse: v1beta1.AdmissionResponse{
			Allowed: true,
		},
	}
}

func denied(msg string) admission.Response {
	return admission.Response{
		AdmissionResponse: v1beta1.AdmissionResponse{
			Allowed: false,
			Result:  &metav1.Status{Message: msg},
		},
	}
}

// opsResourcesExist verify if a resource exist in the namespace,
// it will check its existence during 5 seconds,
// otherwise it will timeout.
func (v *Validator) opsResourcesExist(ctx context.Context, specOpsResource []bdv1.ResourceReference, ns string) (bool, string) {
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

func validateUpdateBlock(update *manifest.Update) error {
	if update == nil {
		return nil
	}
	if _, err := manifest.ExtractWatchTime(update.CanaryWatchTime); err != nil {
		return errors.Wrap(err, "update block has invalid canary_watch_time")
	}
	if _, err := manifest.ExtractWatchTime(update.UpdateWatchTime); err != nil {
		return errors.Wrap(err, "update block has invalid update_watch_time")
	}
	return nil
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
