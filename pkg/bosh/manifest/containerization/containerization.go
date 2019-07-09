// Package containerization loads the kubernetes specifics parts, like
// BOSHContainerization, from the BOSH manifest.
package containerization

import (
	"github.com/ghodss/yaml"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// Manifest is a BOSH deployment manifest
type Manifest struct {
	InstanceGroups []*InstanceGroup `json:"instance_groups,omitempty"`
}

type InstanceGroup struct {
	Jobs []Job `json:"jobs"`
	// for env.bosh.agent.settings.affinity
	Env Env `json:"env,omitempty"`
}

type Job struct {
	Properties JobProperties `json:"properties,omitempty"`
}

type JobProperties struct {
	BOSHContainerization BOSHContainerization `json:"bosh_containerization"`
}

type Env struct {
	BOSH BOSH `json:"bosh,omitempty"`
}

type BOSH struct {
	Agent Agent `json:"agent,omitempty"`
}

type Agent struct {
	Settings Settings `json:"settings,omitempty"`
}

type Settings struct {
	Affinity *corev1.Affinity `json:"affinity,omitempty" yaml:"affinity,omitempty"`
}

// BOSHContainerization represents the special 'bosh_containerization'
// property key. It contains all kubernetes structures we need to add to the BOSH manifest.
type BOSHContainerization struct {
	Consumes         map[string]JobLink `json:"consumes"`
	Instances        []JobInstance      `json:"instances"`
	Release          string             `json:"release"`
	BPM              *bpm.Config        `json:"bpm,omitempty" yaml:"bpm,omitempty"`
	Ports            []Port             `json:"ports"`
	Run              RunConfig          `json:"run"`
	PreRenderScripts []string           `json:"pre_render_scripts" yaml:"pre_render_scripts"`
	Debug            bool               `json:"debug" yaml:"debug"`
}

// Port represents the port to be opened up for this job
type Port struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Internal int    `json:"internal"`
}

// JobInstance for data gathering
type JobInstance struct {
	Address  string                 `json:"address"`
	AZ       string                 `json:"az"`
	ID       string                 `json:"id"`
	Index    int                    `json:"index"`
	Instance int                    `json:"instance"`
	Name     string                 `json:"name"`
	Network  map[string]interface{} `json:"networks"`
	IP       string                 `json:"ip"`
}

// JobLink describes links inside a job properties
// bosh_containerization.
type JobLink struct {
	Instances  []JobInstance          `json:"instances"`
	Properties map[string]interface{} `json:"properties"`
}

// HealthCheck defines liveness and readiness probes for a container
type HealthCheck struct {
	ReadinessProbe *corev1.Probe `json:"readiness"`
	LivenessProbe  *corev1.Probe `json:"liveness"`
}

// RunConfig describes the runtime configuration for this job
type RunConfig struct {
	HealthChecks map[string]HealthCheck `json:"healthcheck"`
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
