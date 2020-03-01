package manifest

import (
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// Quarks represents the special 'quarks' property key.
// It contains all kubernetes structures we need to add to the BOSH manifest.
type Quarks struct {
	Consumes            map[string]JobLink      `json:"consumes"`
	Instances           []JobInstance           `json:"instances"`
	Release             string                  `json:"release"`
	BPM                 *bpm.Config             `json:"bpm,omitempty" yaml:"bpm,omitempty"`
	Ports               []Port                  `json:"ports"`
	Run                 bpm.RunConfig           `json:"run"`
	PreRenderScripts    PreRenderScripts        `json:"pre_render_scripts" yaml:"pre_render_scripts"`
	PostStart           bpm.PostStart           `json:"post_start"`
	Debug               bool                    `json:"debug" yaml:"debug"`
	IsAddon             bool                    `json:"is_addon" yaml:"is_addon"`
	Envs                []corev1.EnvVar         `json:"envs" yaml:"envs"`
	ActivePassiveProbes map[string]corev1.Probe `json:"activePassiveProbes,omitempty"`
}

// Port represents the port to be opened up for this job.
type Port struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Internal int    `json:"internal"`
}

// JobInstance for data gathering.
type JobInstance struct {
	Address   string                 `json:"address"`
	AZ        string                 `json:"az"`
	Index     int                    `json:"index"`
	Instance  int                    `json:"instance"`
	Name      string                 `json:"name"`
	Bootstrap bool                   `json:"bootstrap"`
	ID        string                 `json:"id"`
	Network   map[string]interface{} `json:"networks"`
}

// JobLinkProperties are the properties from the provides section in a job spec manifest
type JobLinkProperties map[string]interface{}

// JobLink describes links inside a job properties quarks.
type JobLink struct {
	Address    string            `json:"address"`
	Instances  []JobInstance     `json:"instances"`
	Properties JobLinkProperties `json:"properties"`
}

// PreRenderScripts describes the different types of scripts that can be run inside a job.
type PreRenderScripts struct {
	BPM        []string `json:"bpm" yaml:"bpm"`
	IgResolver []string `json:"ig_resolver" yaml:"ig_resolver"`
	Jobs       []string `json:"jobs" yaml:"jobs"`
}

// PostStartCondition represents the condition that should succeed in order to execute the
// post-start script. It's often set to be the same as the readiness probe of a job.
type PostStartCondition struct {
	Exec *corev1.ExecAction `json:"exec,omitempty"`
}

// QuarksLink represents the links to share/discover information between BOSH and Kube Native components
type QuarksLink struct {
	Type      string        `json:"type,omitempty"`
	Address   string        `json:"address,omitempty"`
	Instances []JobInstance `json:"instances,omitempty"`
}
