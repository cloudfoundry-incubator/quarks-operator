package controller

import (
	"code.cloudfoundry.org/cf-operator/pkg/controller/boshdeployment"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var addToManagerFuncs = []func(*zap.SugaredLogger, manager.Manager) error{boshdeployment.Add}

// AddToManager adds all Controllers to the Manager
func AddToManager(log *zap.SugaredLogger, m manager.Manager) error {
	for _, f := range addToManagerFuncs {
		if err := f(log, m); err != nil {
			return err
		}
	}
	return nil
}
