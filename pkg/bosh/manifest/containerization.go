package manifest

import (
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// Quarks represents the special 'quarks' property key.
// It contains all kubernetes structures we need to add to the BOSH manifest.
type Quarks struct {
	Consumes         map[string]JobLink `json:"consumes"`
	Instances        []JobInstance      `json:"instances"`
	Release          string             `json:"release"`
	BPM              *bpm.Config        `json:"bpm,omitempty" yaml:"bpm,omitempty"`
	Ports            []Port             `json:"ports"`
	Run              RunConfig          `json:"run"`
	PreRenderScripts []string           `json:"pre_render_scripts" yaml:"pre_render_scripts"`
	Debug            bool               `json:"debug" yaml:"debug"`
	IsAddon          bool               `json:"is_addon" yaml:"is_addon"`
	Envs             []corev1.EnvVar    `json:"envs" yaml:"envs"`
	Privileged       bool               `json:"privileged"`
}

// Port represents the port to be opened up for this job.
type Port struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Internal int    `json:"internal"`
}

// JobInstance for data gathering.
type JobInstance struct {
	Address  string                 `json:"address"`
	AZ       string                 `json:"az"`
	Index    int                    `json:"index"`
	Instance int                    `json:"instance"`
	Name     string                 `json:"name"`
	Network  map[string]interface{} `json:"networks"`
}

// JobLink describes links inside a job properties quarks.
type JobLink struct {
	Address    string                 `json:"address"`
	Instances  []JobInstance          `json:"instances"`
	Properties map[string]interface{} `json:"properties"`
}

// HealthCheck defines liveness and readiness probes for a container.
type HealthCheck struct {
	ReadinessProbe *corev1.Probe `json:"readiness" yaml:"readiness"`
	LivenessProbe  *corev1.Probe `json:"liveness"  yaml:"liveness"`
}

// RunConfig describes the runtime configuration for this job.
type RunConfig struct {
	HealthCheck map[string]HealthCheck `json:"healthcheck" yaml:"healthcheck"`
}
