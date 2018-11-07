package controller

import (
	"code.cloudfoundry.org/cf-operator/pkg/controller/boshdeployment"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var addToManagerFuncs = []func(manager.Manager) error{boshdeployment.Add}

// AddToManager adds all Controllers to the Manager
func AddToManager(m manager.Manager) error {
	for _, f := range addToManagerFuncs {
		if err := f(m); err != nil {
			return err
		}
	}
	return nil
}
