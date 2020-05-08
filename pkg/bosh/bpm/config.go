package bpm

import (
	"reflect"
	"sort"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// Hooks from a BPM config
type Hooks struct {
	PreStart string `yaml:"pre_start,omitempty" json:"pre_start,omitempty"`
}

// Limits from a BPM config
type Limits struct {
	Memory    string `yaml:"memory,omitempty" json:"memory,omitempty"`
	OpenFiles int    `yaml:"open_files,omitempty" json:"open_files,omitempty"`
	Processes int    `yaml:"processes,omitempty" json:"processes,omitempty"`
}

// Volume from a BPM config
type Volume struct {
	Path            string `yaml:"path,omitempty" json:"path,omitempty"`
	Writable        bool   `yaml:"writable,omitempty" json:"writable,omitempty"`
	AllowExecutions bool   `yaml:"allow_executions,omitempty" json:"allow_executions,omitempty"`
	MountOnly       bool   `yaml:"mount_only,omitempty" json:"mount_only,omitempty"`
}

// Unsafe from a BPM config
type Unsafe struct {
	Privileged          bool     `yaml:"privileged,omitempty" json:"privileged,omitempty"`
	UnrestrictedVolumes []Volume `yaml:"unrestricted_volumes,omitempty" json:"unrestricted_volumes,omitempty"`
}

// Process from a BPM config
type Process struct {
	Name              string              `yaml:"name,omitempty" json:"name,omitempty"`
	Executable        string              `yaml:"executable,omitempty" json:"executable,omitempty"`
	Args              []string            `yaml:"args,omitempty" json:"args,omitempty"`
	Env               map[string]string   `yaml:"env,omitempty" json:"env,omitempty"`
	Workdir           string              `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	Hooks             Hooks               `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	Capabilities      []string            `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Limits            Limits              `yaml:"limits,omitempty" json:"limits,omitempty"`
	Requests          corev1.ResourceList `json:"requests,omitempty" protobuf:"bytes,2,rep,name=requests,casttype=ResourceList,castkey=ResourceName"`
	EphemeralDisk     *bool               `yaml:"ephemeral_disk,omitempty" json:"ephemeral_disk,omitempty"`
	PersistentDisk    *bool               `yaml:"persistent_disk,omitempty" json:"persistent_disk,omitempty"`
	AdditionalVolumes []Volume            `yaml:"additional_volumes,omitempty" json:"additional_volumes,omitempty"`
	Unsafe            Unsafe              `yaml:"unsafe,omitempty" json:"unsafe,omitempty"`
}

// Port represents the port to be opened up for this job only for tracing changes.
type Port struct {
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Internal int    `json:"internal"`
}

// Config represent a BPM configuration
type Config struct {
	Processes           []Process `yaml:"processes,omitempty" json:"processes,omitempty"`
	UnsupportedTemplate bool      `json:"unsupported_template"`

	// Bind these to reflect quarks properties update
	Ports               []Port                  `yaml:"ports,omitempty" json:"ports,omitempty"`
	Run                 RunConfig               `json:"run"`
	PostStart           PostStart               `json:"post_start"`
	Debug               bool                    `json:"debug"`
	ActivePassiveProbes map[string]corev1.Probe `json:"activePassiveProbes"`
}

// RunConfig describes the runtime configuration for this job.
type RunConfig struct {
	HealthCheck     map[string]HealthCheck  `json:"healthcheck" yaml:"healthcheck"`
	SecurityContext *corev1.SecurityContext `json:"security_context" yaml:"security_context"`
}

// HealthCheck defines liveness and readiness probes for a container.
type HealthCheck struct {
	ReadinessProbe *corev1.Probe `json:"readiness" yaml:"readiness"`
	LivenessProbe  *corev1.Probe `json:"liveness"  yaml:"liveness"`
}

// PostStart allows post-start specifics to be passed through the manifest.
type PostStart struct {
	Condition *PostStartCondition `json:"condition,omitempty"`
}

// PostStartCondition represents the condition that should succeed in order to execute the
// post-start script. It's often set to be the same as the readiness probe of a job.
type PostStartCondition struct {
	Exec *corev1.ExecAction `json:"exec,omitempty"`
}

// Configs holds a collection of BPM configurations by their according job
type Configs map[string]Config

// IsActivePassiveModel indicates whether these bpm configs contain ActivePassiveProbes
func (cs Configs) IsActivePassiveModel() (isActivePassiveModel bool) {
	for _, config := range cs {
		if len(config.ActivePassiveProbes) > 0 {
			isActivePassiveModel = true
		}
	}

	return
}

// ActivePassiveProbes returns all activePassive probes defined in the bpm configs
func (cs Configs) ActivePassiveProbes() map[string]corev1.Probe {
	probes := map[string]corev1.Probe{}
	for _, config := range cs {
		for container, probe := range config.ActivePassiveProbes {
			probes[container] = probe
		}
	}
	return probes
}

// ServicePorts returns the service ports defined in the bpm configs
func (cs Configs) ServicePorts() []corev1.ServicePort {
	ports := []corev1.ServicePort{}

	for _, c := range cs {
		for _, port := range c.Ports {
			ports = append(ports, corev1.ServicePort{
				Name:     port.Name,
				Protocol: corev1.Protocol(port.Protocol),
				Port:     int32(port.Internal),
			})
		}
	}
	return ports
}

// NewConfig creates a new Config object from the yaml
func NewConfig(data []byte) (Config, error) {
	config := Config{}
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, errors.Wrapf(err, "Unmarshalling data %s failed", string(data))
	}
	return config, nil
}

type nullTransformer struct {
}

func (t *nullTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ.Kind() == reflect.Ptr {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() && !src.IsNil() {
				dst.Set(src)
			}
			return nil
		}
	}
	return nil
}

// MergeProcesses adds and updates the preset processes and returns a new list
func (c Config) MergeProcesses(presetProcesses []Process) ([]Process, error) {
	renderedProcesses := c.Processes
	for _, process := range presetProcesses {
		index, exist := indexOfBPMProcess(renderedProcesses, process.Name)
		if exist {
			err := mergo.MergeWithOverwrite(&renderedProcesses[index], process, mergo.WithTransformers(&nullTransformer{}))
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to merge bpm process information for preset process %s", process.Name)
			}
		} else {
			renderedProcesses = append(renderedProcesses, process)
		}
	}
	return renderedProcesses, nil
}

// indexOfBPMProcess will return the first index at which a given process name can be found in the []bpm.Process.
// Return -1 if not find valid version
func indexOfBPMProcess(processes []Process, processName string) (int, bool) {
	for i, process := range processes {
		if process.Name == processName {
			return i, true
		}
	}
	return -1, false
}

// ValidateProcesses checks if all processes have an executable
func (c Config) ValidateProcesses() error {
	for _, process := range c.Processes {
		if process.Executable == "" {
			return errors.Errorf("no executable specified for process %s", process.Name)
		}
	}
	return nil
}

// NewEnvs returns a list of k8s env vars, based on the bpm envs, overwritten
// by the overrides list passed to the function.
func (p *Process) NewEnvs(overrides []corev1.EnvVar) []corev1.EnvVar {
	seen := make(map[string]corev1.EnvVar)

	for name, value := range p.Env {
		seen[name] = corev1.EnvVar{Name: name, Value: value}
	}

	for _, env := range overrides {
		seen[env.Name] = env
	}

	result := make([]corev1.EnvVar, 0, len(seen))
	for _, value := range seen {
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// UpdateEnv adds the overrides env vars to the env list of the bpm process
func (p *Process) UpdateEnv(overrides []corev1.EnvVar) {
	if p.Env == nil {
		p.Env = map[string]string{}
	}
	for _, env := range overrides {
		if env.Value == "" && env.ValueFrom != nil {
			p.Env[env.Name] = env.ValueFrom.String()
		} else {
			p.Env[env.Name] = env.Value
		}
	}
}
