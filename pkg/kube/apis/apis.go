package apis

const (
	// GroupName defines the kube group name for all controllers
	// It's what's used when you specify the apiVersion of a resource.
	// e.g.:
	//
	//   ---
	//   apiVersion: fissile.cloudfoundry.org/v1alpha1
	//   kind: BOSHDeployment
	//   ...
	GroupName = "fissile.cloudfoundry.org"
)
