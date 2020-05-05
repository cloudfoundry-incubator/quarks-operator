package manifest

import (
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpm"
)

// BPMInfo contains custom information about
// instance group which matters for quarksStatefulSet pods
// such as AZ's, instance group count and BPM Configs
type BPMInfo struct {
	InstanceGroup BPMInstanceGroup `json:"instance_group,omitempty"`
	Configs       bpm.Configs      `json:"configs,omitempty"`
	Variables     []Variable       `json:"variables,omitempty"`
}

// BPMInstanceGroup is a custom instance group spec
// that should be included in the BPM secret created
// by the bpm quarksJob.
type BPMInstanceGroup struct {
	Name      string   `json:"name"`
	Instances int      `json:"instances"`
	AZs       []string `json:"azs"`
	Env       AgentEnv `json:"env,omitempty"`
}
