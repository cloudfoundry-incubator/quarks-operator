package converter

import "code.cloudfoundry.org/cf-operator/pkg/kube/util/names"

// operatorDockerImage is the location of the operators own docker image
var operatorDockerImage = ""

// SetupOperatorDockerImage initializes the package scoped variable
func SetupOperatorDockerImage(org, repo, tag string) (err error) {
	operatorDockerImage, err = names.GetDockerSourceName(org, repo, tag)
	return err
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return operatorDockerImage
}
