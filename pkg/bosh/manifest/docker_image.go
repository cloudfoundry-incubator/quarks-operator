package manifest

var (
	// DockerImageOrganization is the organization which provides the operator image
	DockerImageOrganization = ""
	// DockerImageRepository is the repository which provides the operator image
	DockerImageRepository = ""
	// DockerImageTag is the tag of the operator image
	DockerImageTag = ""
)

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return DockerImageOrganization + "/" + DockerImageRepository + ":" + DockerImageTag
}
