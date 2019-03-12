package controllers

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"

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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
)

var addToManagerFuncs = []func(*zap.SugaredLogger, *context.Config, manager.Manager) error{
	boshdeployment.Add,
	extendedjob.AddTrigger,
	extendedjob.AddErrand,
	extendedjob.AddOutput,
	extendedsecret.Add,
	extendedstatefulset.Add,
}

var addToSchemes = runtime.SchemeBuilder{
	bdcv1.AddToScheme,
	ejv1.AddToScheme,
	esv1.AddToScheme,
	essv1.AddToScheme,
}

var addHookFuncs = []func(*zap.SugaredLogger, *context.Config, manager.Manager, *webhook.Server) (*admission.Webhook, error){
	extendedstatefulset.AddPod,
}

// AddToManager adds all Controllers to the Manager
func AddToManager(log *zap.SugaredLogger, ctrConfig *context.Config, m manager.Manager) error {
	for _, f := range addToManagerFuncs {
		if err := f(log, ctrConfig, m); err != nil {
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
func AddHooks(log *zap.SugaredLogger, ctrConfig *context.Config, m manager.Manager, generator credsgen.Generator) error {
	log.Infof("Setting up webhook server on %s:%d", ctrConfig.WebhookServerHost, ctrConfig.WebhookServerPort)

	webhookConfig := NewWebhookConfig(log, m.GetClient(), ctrConfig, generator, "cf-operator-mutating-hook")

	disableConfigInstaller := true
	hookServer, err := webhook.NewServer("cf-operator", m, webhook.ServerOptions{
		Port:    ctrConfig.WebhookServerPort,
		CertDir: webhookConfig.CertDir,
		DisableWebhookConfigInstaller: &disableConfigInstaller,
		BootstrapOptions: &webhook.BootstrapOptions{
			MutatingWebhookConfigName: webhookConfig.ConfigName,
			Host: &ctrConfig.WebhookServerHost,
			// The user should probably be able to use a service instead.
			// Service: ??
		},
	})
	if err != nil {
		return errors.Wrap(err, "unable to create a new webhook server")
	}

	webhooks := []*admission.Webhook{}
	for _, f := range addHookFuncs {
		wh, err := f(log, ctrConfig, m, hookServer)
		if err != nil {
			return err
		}
		webhooks = append(webhooks, wh)
	}

	err = webhookConfig.setupCertificate()
	if err != nil {
		return errors.Wrap(err, "setting up the webhook server certificate")
	}
	err = webhookConfig.generateWebhookServerConfig(webhooks)
	if err != nil {
		return errors.Wrap(err, "generating the webhook server configuration")
	}

	return err
}
