package bpmconverter

const (
	// EnvInstanceGroupName is a key for the container Env identifying the
	// instance group that container is started for (CLI)
	EnvInstanceGroupName = "INSTANCE_GROUP_NAME"
	// EnvDeploymentName is the name of the BOSH deployment
	EnvDeploymentName = "DEPLOYMENT_NAME"
	// EnvBOSHManifestPath is a key for the container Env pointing to the BOSH manifest (CLI)
	EnvBOSHManifestPath = "BOSH_MANIFEST_PATH"
	// PodIPEnvVar is the environment variable containing status.podIP used to render BOSH spec.ip. (CLI)
	PodIPEnvVar = "POD_IP"
)
