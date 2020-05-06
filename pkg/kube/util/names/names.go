package names

import (
	"fmt"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// SecretVariableName generates a valid secret name for a given name
// `var-<name>`
func SecretVariableName(name string) string {
	secretType := bdv1.DeploymentSecretTypeVariable
	if name == "" {
		name = secretType.String()
	} else {
		name = fmt.Sprintf("%s-%s", secretType, name)
	}
	return names.SanitizeSubdomain(name)
}

// InstanceGroupSecretName returns the name of a k8s secret:
// `<secretType>.<instance-group>-v<version>` secret.
//
// These secrets are created by QuarksJob and mounted on containers, e.g.
// for the template rendering.
func InstanceGroupSecretName(igName string, version string) string {
	prefix := bdv1.DeploymentSecretTypeInstanceGroupResolvedProperties.Prefix()
	finalName := names.SanitizeSubdomain(prefix + igName)

	if version != "" {
		finalName = fmt.Sprintf("%s-v%s", finalName, version)
	}

	return finalName
}

// ServiceName returns the service name for a deployment
func ServiceName(igName string, maxLength int) string {
	s := names.DNSLabelSafe(igName)
	return names.TruncateMD5(s, maxLength)
}
