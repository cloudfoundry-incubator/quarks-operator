package bpm

import (
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
	Name              string            `yaml:"name,omitempty" json:"name,omitempty"`
	Executable        string            `yaml:"executable,omitempty" json:"executable,omitempty"`
	Args              []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env               map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Workdir           string            `yaml:"workdir,omitempty" json:"workdir,omitempty"`
	Hooks             Hooks             `yaml:"hooks,omitempty" json:"hooks,omitempty"`
	Capabilities      []string          `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Limits            Limits            `yaml:"limits,omitempty" json:"limits,omitempty"`
	EphemeralDisk     bool              `yaml:"ephemeral_disk,omitempty" json:"ephemeral_disk,omitempty"`
	PersistentDisk    bool              `yaml:"persistent_disk,omitempty" json:"persistent_disk,omitempty"`
	AdditionalVolumes []Volume          `yaml:"additional_volumes,omitempty" json:"additional_volumes,omitempty"`
	Unsafe            Unsafe            `yaml:"unsafe,omitempty" json:"unsafe,omitempty"`
}

// Config represent a BPM configuration
type Config struct {
	Processes           []Process `yaml:"processes,omitempty" json:"processes,omitempty"`
	UnsupportedTemplate bool      `json:"unsupported_template"`
}

// Configs holds a collection of BPM configurations by their according job
type Configs map[string]Config

// NewConfig creates a new Config object from the yaml
func NewConfig(data []byte) (Config, error) {
	config := Config{}
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}
