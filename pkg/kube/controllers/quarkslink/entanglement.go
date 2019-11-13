package quarkslink

import (
	"fmt"
	"regexp"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	corev1 "k8s.io/api/core/v1"
)

const (
	// DeploymentKey is the key to retrieve the name of the deployment,
	// which provides the variables for the pod
	DeploymentKey = "quarks.cloudfoundry.org/deployment"
	// ConsumesKey is the key for identifying the provider to be consumed, in the
	// format of 'type.job'
	ConsumesKey = "quarks.cloudfoundry.org/consumes"
)

func validEntanglement(annotations map[string]string) bool {
	if annotations[DeploymentKey] != "" && annotations[ConsumesKey] != "" {
		return true
	}
	return false
}

type entanglement struct {
	deployment string
	consumes   string
}

func newEntanglement(obj map[string]string) entanglement {
	e := entanglement{
		deployment: obj[DeploymentKey],
		consumes:   obj[ConsumesKey],
	}
	return e
}

func (e entanglement) fulfilledBy(secret corev1.Secret) bool {
	// secret has a deployment label
	entanglementDeployment, found := secret.Labels[manifest.LabelDeploymentName]
	if !found {
		return false
	}

	// deployment label matches entanglements'
	if entanglementDeployment != e.deployment {
		return false
	}

	// secret name is a valid quarks link name and matches deployment
	var regex = regexp.MustCompile(fmt.Sprintf("^link-%s-[a-z0-9-]*$", e.deployment))
	if !regex.MatchString(secret.Name) {
		return false
	}

	// secret contains the requested properties
	if _, found := secret.Data[e.consumes]; !found {
		return false
	}
	return true
}
