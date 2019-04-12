package controllers

import (
	"context"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	machinerytypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

var addToManagerFuncs = []func(context.Context, *config.Config, manager.Manager) error{
	boshdeployment.AddDeployment,
	extendedjob.AddTrigger,
	extendedjob.AddErrand,
	extendedjob.AddJob,
	extendedjob.AddOwnership,
	extendedsecret.Add,
	extendedstatefulset.Add,
}

var addToSchemes = runtime.SchemeBuilder{
	bdcv1.AddToScheme,
	ejv1.AddToScheme,
	esv1.AddToScheme,
	essv1.AddToScheme,
}

var addHookFuncs = []func(*zap.SugaredLogger, *config.Config, manager.Manager, *webhook.Server) (*admission.Webhook, error){
	extendedstatefulset.AddPod,
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

	webhookConfig := NewWebhookConfig(m.GetClient(), config, generator, "cf-operator-mutating-hook-"+config.Namespace)

	disableConfigInstaller := true
	hookServer, err := webhook.NewServer("cf-operator", m, webhook.ServerOptions{
		Port:                          config.WebhookServerPort,
		CertDir:                       webhookConfig.CertDir,
		DisableWebhookConfigInstaller: &disableConfigInstaller,
		BootstrapOptions: &webhook.BootstrapOptions{
			MutatingWebhookConfigName: webhookConfig.ConfigName,
			Host:                      &config.WebhookServerHost,
			// The user should probably be able to use a service instead.
			// Service: ??
		},
	})

	if err != nil {
		return errors.Wrap(err, "unable to create a new webhook server")
	}

	log := ctxlog.ExtractLogger(ctx)
	webhooks := []*admission.Webhook{}
	for _, f := range addHookFuncs {
		wh, err := f(log, config, m, hookServer)
		if err != nil {
			return err
		}
		webhooks = append(webhooks, wh)
	}

	err = setOperatorNamespaceLabel(ctx, config, m.GetClient())
	if err != nil {
		return errors.Wrap(err, "setting the operator namespace label")
	}

	err = webhookConfig.setupCertificate(ctx)
	if err != nil {
		return errors.Wrap(err, "setting up the webhook server certificate")
	}
	err = webhookConfig.generateWebhookServerConfig(ctx, webhooks)
	if err != nil {
		return errors.Wrap(err, "generating the webhook server configuration")
	}

	return err
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
