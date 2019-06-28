package manifest

import (
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// BOSHContainerization represents the special 'bosh_containerization'
// property key
type BOSHContainerization struct {
	Consumes  map[string]JobLink `yaml:"consumes"`
	Instances []JobInstance      `yaml:"instances"`
	Release   string             `yaml:"release"`
	BPM       *bpm.Config        `yaml:"bpm,omitempty"`
	Ports     []Port             `yaml:"ports"`
	Run       RunConfig          `yaml:"run"`
	Patches   []string           `yaml:"patches"`
}

// Port represents the port to be opened up for this job
type Port struct {
	Name     string `yaml:"name"`
	Protocol string `yaml:"protocol"`
	Internal int    `yaml:"internal"`
}

// JobInstance for data gathering
type JobInstance struct {
	Address  string                 `yaml:"address"`
	AZ       string                 `yaml:"az"`
	ID       string                 `yaml:"id"`
	Index    int                    `yaml:"index"`
	Instance int                    `yaml:"instance"`
	Name     string                 `yaml:"name"`
	Network  map[string]interface{} `yaml:"networks"`
	IP       string                 `yaml:"ip"`
}

// JobLink describes links inside a job properties
// bosh_containerization.
type JobLink struct {
	Instances  []JobInstance          `yaml:"instances"`
	Properties map[string]interface{} `yaml:"properties"`
}

// HealthCheck defines liveness and readiness probes for a container
type HealthCheck struct {
	ReadinessProbe *corev1.Probe `yaml:"readiness"`
	LivenessProbe  *corev1.Probe `yaml:"liveness"`
}

// RunConfig describes the runtime configuration for this job
type RunConfig struct {
	HealthChecks map[string]HealthCheck `yaml:"healthcheck"`
}
