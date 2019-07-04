// Package containerization loads the kubernetes specifics parts, like
// BOSHContainerization, from the BOSH manifest.
package containerization

import (
	yaml "gopkg.in/yaml.v2"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// Manifest is a BOSH deployment manifest
type Manifest struct {
	InstanceGroups []*InstanceGroup `yaml:"instance_groups,omitempty"`
}
type InstanceGroup struct {
	Jobs []Job `yaml:"jobs"`
}
type Job struct {
	Properties JobProperties `yaml:"properties,omitempty"`
}

type JobProperties struct {
	BOSHContainerization BOSHContainerization `yaml:"bosh_containerization"`
}

// BOSHContainerization represents the special 'bosh_containerization'
// property key. It contains all kubernetes structures we need to add to the BOSH manifest.
type BOSHContainerization struct {
	Consumes         map[string]JobLink `yaml:"consumes"`
	Instances        []JobInstance      `yaml:"instances"`
	Release          string             `yaml:"release"`
	BPM              *bpm.Config        `yaml:"bpm,omitempty"`
	Ports            []Port             `yaml:"ports"`
	Run              RunConfig          `yaml:"run"`
	PreRenderScripts []string           `yaml:"pre_render_scripts"`
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

// LoadKubeYAML is a special loader, since the YAML is already compatible to
// k8s structures without further transformation.
func LoadKubeYAML(data []byte) (*Manifest, error) {
	m := &Manifest{}
	err := yaml.Unmarshal(data, m)
	if err != nil {
		return nil, err
	}
	return m, nil
}
