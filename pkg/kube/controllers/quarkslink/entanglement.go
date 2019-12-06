package quarkslink

const (
	// DeploymentKey is the key to retrieve the name of the deployment,
	// which provides the variables for the pod
	DeploymentKey = "quarks.cloudfoundry.org/deployment"
	// ConsumesKey is the key to identify the job, which properties are consumed,
	// which is of 'type.job'
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
