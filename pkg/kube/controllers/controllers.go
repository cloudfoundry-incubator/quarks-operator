package controllers

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ess "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
)

var addToManagerFuncs = []func(*zap.SugaredLogger, manager.Manager) error{
	boshdeployment.Add,
}

var addToSchemes = runtime.SchemeBuilder{
	bdc.AddToScheme,
	ess.AddToScheme,
}

// AddToManager adds all Controllers to the Manager
func AddToManager(log *zap.SugaredLogger, m manager.Manager) error {
	for _, f := range addToManagerFuncs {
		if err := f(log, m); err != nil {
			return err
		}
	}
	return nil
}

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return addToSchemes.AddToScheme(s)
}
