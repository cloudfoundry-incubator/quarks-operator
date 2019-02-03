package controllers

import (
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
)

var addToManagerFuncs = []func(*zap.SugaredLogger, manager.Manager) error{
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
