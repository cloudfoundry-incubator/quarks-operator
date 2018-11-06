package controller

import (
	"code.cloudfoundry.org/cf-operator/pkg/controller/boshdeployment"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, boshdeployment.Add)
}
