package controllers

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	machinerytypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	wh "code.cloudfoundry.org/cf-operator/pkg/kube/util/webhook"
)

const (
	// HTTPReadyzEndpoint route
	HTTPReadyzEndpoint = "/readyz"
	// WebhookConfigPrefix is the prefix for the dir containing the webhook SSL certs
	WebhookConfigPrefix = "cf-operator-hook-"
	// WebhookConfigDir contains the dir with the webhook SSL certs
	WebhookConfigDir = "/tmp"
)

// Theses funcs construct controllers and add them to the controller-runtime
// manager. The manager will set fields on the controllers and start them, when
// itself is started.
var addToManagerFuncs = []func(context.Context, *config.Config, manager.Manager) error{
	boshdeployment.AddDeployment,
	boshdeployment.AddGeneratedVariable,
	boshdeployment.AddBPM,
	extendedjob.AddErrand,
	extendedjob.AddJob,
	extendedsecret.AddExtendedSecret,
	extendedsecret.AddCertificateSigningRequest,
	extendedstatefulset.AddExtendedStatefulSet,
	extendedstatefulset.AddStatefulSetCleanup,
}

var addToSchemes = runtime.SchemeBuilder{
	bdv1.AddToScheme,
	ejv1.AddToScheme,
	esv1.AddToScheme,
	essv1.AddToScheme,
}

var validatingHookFuncs = []func(*zap.SugaredLogger, *config.Config) *wh.OperatorWebhook{
	boshdeployment.NewBOSHDeploymentValidator,
}

var mutatingHookFuncs = []func(*zap.SugaredLogger, *config.Config) *wh.OperatorWebhook{
	extendedstatefulset.NewExtendedStatefulsetPodMutator,
}

// AddToManager adds all Controllers to the Manager
func AddToManager(ctx context.Context, config *config.Config, m manager.Manager) error {
	for _, f := range addToManagerFuncs {
		if err := f(ctx, config, m); err != nil {
			return err
		}
	}
	return nil
}

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return addToSchemes.AddToScheme(s)
}

// AddHooks adds all web hooks to the Manager
func AddHooks(ctx context.Context, config *config.Config, m manager.Manager, generator credsgen.Generator) error {
	ctxlog.Infof(ctx, "Setting up webhook server on %s:%d", config.WebhookServerHost, config.WebhookServerPort)

	ctxlog.Info(ctx, "Setting a cf-operator namespace label")
	err := setOperatorNamespaceLabel(ctx, config, m.GetClient())
	if err != nil {
		return errors.Wrap(err, "setting the operator namespace label")
	}

	webhookConfig := NewWebhookConfig(m.GetClient(), config, generator, WebhookConfigPrefix+config.Namespace)

	hookServer := m.GetWebhookServer()
	hookServer.CertDir = webhookConfig.CertDir

	hookServer.Register(HTTPReadyzEndpoint, ordinaryHTTPHandler())

	validatingWebhooks := make([]*wh.OperatorWebhook, len(validatingHookFuncs))
	log := ctxlog.ExtractLogger(ctx)
	for idx, f := range validatingHookFuncs {
		hook := f(log, config)
		validatingWebhooks[idx] = hook
		hookServer.Register(hook.Path, hook.Webhook)
	}

	mutatingWebhooks := make([]*wh.OperatorWebhook, len(mutatingHookFuncs))
	for idx, f := range mutatingHookFuncs {
		hook := f(log, config)
		mutatingWebhooks[idx] = hook
		hookServer.Register(hook.Path, hook.Webhook)
	}

	ctxlog.Info(ctx, "generating webhook certificates")
	err = webhookConfig.setupCertificate(ctx)
	if err != nil {
		return errors.Wrap(err, "setting up the webhook server certificate")
	}

	ctxlog.Info(ctx, "generating validating webhook server configuration")
	err = webhookConfig.generateValidationWebhookServerConfig(ctx, validatingWebhooks)
	if err != nil {
		return errors.Wrap(err, "generating the validating webhook server configuration")
	}

	ctxlog.Info(ctx, "generating mutating webhook server configuration")
	err = webhookConfig.generateMutationWebhookServerConfig(ctx, mutatingWebhooks)
	if err != nil {
		return errors.Wrap(err, "generating the webhook server configuration")
	}

	return nil
}

func ordinaryHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func setOperatorNamespaceLabel(ctx context.Context, config *config.Config, c client.Client) error {
	ns := &unstructured.Unstructured{}
	ns.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Kind:    "Namespace",
		Version: "v1",
	})
	err := c.Get(ctx, machinerytypes.NamespacedName{Name: config.Namespace}, ns)

	if err != nil {
		return errors.Wrap(err, "getting the namespace object")
	}

	labels := ns.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels["cf-operator-ns"] = config.Namespace
	ns.SetLabels(labels)
	err = c.Update(ctx, ns)

	if err != nil {
		return errors.Wrap(err, "updating the namespace object")
	}

	return nil
}
