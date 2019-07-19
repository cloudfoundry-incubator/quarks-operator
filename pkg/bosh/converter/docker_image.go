package converter

// DockerSource describes a docker image on docker hub
type DockerSource struct {
	// Organization is the organization which provides the operator image
	Organization string
	// Repository is the repository which provides the operator image
	Repository string
	// Tag is the tag of the operator image
	Tag string
}

// GetName returns the name of the docker image
// More info: https://kubernetes.io/docs/concepts/containers/images
func (d DockerSource) GetName() string {
	return d.Organization + "/" + d.Repository + ":" + d.Tag
}

// OperatorDockerImage is the location of the operators own docker image
var OperatorDockerImage = &DockerSource{}

// SetupOperatorDockerImage initializes the package scoped variable
func SetupOperatorDockerImage(org, repo, tag string) {
	OperatorDockerImage = &DockerSource{
		Organization: org,
		Repository:   repo,
		Tag:          tag,
	}
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return OperatorDockerImage.GetName()
}
