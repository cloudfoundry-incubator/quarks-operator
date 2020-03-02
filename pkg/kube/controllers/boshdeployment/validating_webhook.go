package boshdeployment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"k8s.io/api/admission/v1beta1"
	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/statefulset"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/boshdns"
	wh "code.cloudfoundry.org/cf-operator/pkg/kube/util/webhook"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/withops"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// NewBOSHDeploymentValidator creates a validating hook for BOSHDeployment and adds it to the Manager
func NewBOSHDeploymentValidator(log *zap.SugaredLogger, config *config.Config) *wh.OperatorWebhook {
	log.Info("Setting up validator for BOSHDeployment")

	boshDeploymentValidator := NewValidator(log, config)

	globalScopeType := admissionregistration.ScopeType("*")
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
				wh.LabelWatchNamespace: config.OperatorNamespace,
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
	validationLog := log.Named("boshdeployment-validator").Desugar().WithOptions(zap.AddCallerSkip(-1)).Sugar()
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
func (v *Validator) Handle(ctx context.Context, req admission.Request) admission.Response {
	boshDeployment := &bdv1.BOSHDeployment{}

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

	v.log.Infof("Verifying dependencies for deployment '%s'", boshDeployment.Name)
	withops := withops.NewResolver(
		v.client,
		func() withops.Interpolator { return withops.NewInterpolator() },
		func(deploymentName string, m bdm.Manifest) (withops.DomainNameService, error) {
			return boshdns.NewDNS(deploymentName, m)
		},
	)
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

	v.log.Infof("Resolving deployment '%s'", boshDeployment.Name)
	manifest, _, err := withops.ManifestDetailed(boshDeployment, boshDeployment.GetNamespace())
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
	err = validateUpdateBlock(*manifest)
	if err != nil {
		return admission.Response{
			AdmissionResponse: v1beta1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Message: fmt.Sprintf("Failed to validate update block: %s", err.Error()),
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

func validateUpdateBlock(manifest manifest.Manifest) error {
	if manifest.Update == nil {
		return nil
	}
	if _, err := statefulset.ExtractWatchTime(manifest.Update.CanaryWatchTime, "canary_watch_time"); err != nil {
		return err
	}
	_, err := statefulset.ExtractWatchTime(manifest.Update.UpdateWatchTime, "update_watch_time")
	return err
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
