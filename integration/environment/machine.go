package environment

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// Machine produces and destroys resources for tests
type Machine struct {
	machine.Machine

	VersionedClientset *versioned.Clientset
}
