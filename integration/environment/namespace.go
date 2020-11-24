package environment

import (
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-utils/pkg/monitorednamespace"
	utils "code.cloudfoundry.org/quarks-utils/testing/integration"
)

// SetupNamespace creates the namespace and the clientsets and prepares the teardowm
func (e *Environment) SetupNamespace() error {
	return utils.SetupNamespace(e.Environment, e.Machine.Machine,
		map[string]string{
			monitorednamespace.LabelNamespace:  e.Config.MonitoredID,
			qjv1a1.LabelServiceAccount:         persistOutputServiceAccount,
			boshdns.CorednsServiceAccountLabel: "coredns-quarks",
		})
}
