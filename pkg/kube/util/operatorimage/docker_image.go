package operatorimage

import (
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// operatorDockerImage is the location of the operators own docker image
var operatorDockerImage string
var operatorImagePullPolicy corev1.PullPolicy

// SetupOperatorDockerImage initializes the package scoped variable
func SetupOperatorDockerImage(org, repo, tag string, pullPolicy corev1.PullPolicy) error {
	image, err := names.GetDockerSourceName(org, repo, tag)
	if err != nil {
		return err
	}

	// setup quarks job docker image, too.
	// will have to change this once the persist command is moved to quarks-job
	if err := config.SetupOperatorDockerImage(org, repo, tag); err != nil {
		return err
	}

	operatorDockerImage = image
	if pullPolicy == "" {
		operatorImagePullPolicy = corev1.PullIfNotPresent
	} else {
		operatorImagePullPolicy = pullPolicy
	}
	config.SetupOperatorImagePullPolicy(string(operatorImagePullPolicy))

	return nil
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return operatorDockerImage
}

// GetOperatorImagePullPolicy returns the image pull policy to be used for generated pods
func GetOperatorImagePullPolicy() corev1.PullPolicy {
	return operatorImagePullPolicy
}
