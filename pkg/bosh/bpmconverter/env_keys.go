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
	// EnvInitialRollout is set to "false" if this is not the first time the instance group has run
	EnvInitialRollout = "INITIAL_ROLLOUT"
	// EnvPodOrdinal is the environment variable which holds the pods startup index
	EnvPodOrdinal = "POD_ORDINAL"

	// EnvReplicas is set to 1
	EnvReplicas = "REPLICAS"
	// EnvAzIndex is set by available zone index
	EnvAzIndex = "AZ_INDEX"
)
