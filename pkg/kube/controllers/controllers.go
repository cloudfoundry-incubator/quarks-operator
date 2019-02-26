package controllers

import (
	"os"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllersconfig"
)

var addToManagerFuncs = []func(*zap.SugaredLogger, *controllersconfig.ControllersConfig, manager.Manager) error{
	boshdeployment.Add,
	extendedjob.Add,
	extendedjob.AddErrand,
	extendedsecret.Add,
	extendedstatefulset.Add,
}

var addToSchemes = runtime.SchemeBuilder{
	bdcv1.AddToScheme,
	ejv1.AddToScheme,
	esv1.AddToScheme,
	essv1.AddToScheme,
}

var addHookFuncs = []func(*zap.SugaredLogger, *controllersconfig.ControllersConfig, manager.Manager, *webhook.Server) error{
	extendedstatefulset.AddPod,
}

// AddToManager adds all Controllers to the Manager
func AddToManager(log *zap.SugaredLogger, ctrConfig *controllersconfig.ControllersConfig, m manager.Manager) error {
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
func AddHooks(log *zap.SugaredLogger, ctrConfig *controllersconfig.ControllersConfig, m manager.Manager) error {
	log.Info("Setting up webhook server")

	var host *string
	hostEnv := os.ExpandEnv("${OPERATOR_WEBHOOK_HOST}")
	host = &hostEnv

	if strings.TrimSpace(hostEnv) == "" {
		host = nil
	}

	// TODO: port should be configurable
	hookServer, err := webhook.NewServer("cf-operator", m, webhook.ServerOptions{
		Port:    2999,
		CertDir: "/tmp/cert",
		BootstrapOptions: &webhook.BootstrapOptions{
			// This should be properly configurable, and the user
			// should be able to use a service instead.
			Host: host,
			// Service: ??
		},
	})

	if err != nil {
		return errors.Wrap(err, "unable to create a new webhook server")
	}

	for _, f := range addHookFuncs {
		if err := f(log, ctrConfig, m, hookServer); err != nil {
			return err
		}
	}
	return nil
}
