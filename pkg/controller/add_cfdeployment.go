package controller

import (
	"github.com/manno/cf-operator/pkg/controller/cfdeployment"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, cfdeployment.Add)
}
