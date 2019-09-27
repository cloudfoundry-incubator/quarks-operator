package converter

import (
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// operatorDockerImage is the location of the operators own docker image
var operatorDockerImage = ""

// SetupOperatorDockerImage initializes the package scoped variable
func SetupOperatorDockerImage(org, repo, tag string) error {
	var err error
	operatorDockerImage, err = names.GetDockerSourceName(org, repo, tag)
	if err != nil {
		return err
	}

	// setup quarks job docker image, too.
	// will have to change this once the persist command is moved to quarks-job
	return extendedjob.SetupOperatorDockerImage(org, repo, tag)
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return operatorDockerImage
}
