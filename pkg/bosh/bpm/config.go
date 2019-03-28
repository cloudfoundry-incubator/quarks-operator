package bpm

import yaml "gopkg.in/yaml.v2"

// Hooks from a BPM config
type Hooks struct {
	PreStart string `yaml:"pre_start,omitempty"`
}

// Limits from a BPM config
type Limits struct {
	Memory    string `yaml:"memory,omitempty"`
	OpenFiles int    `yaml:"open_files,omitempty"`
	Processes int    `yaml:"processes,omitempty"`
}

// Volume from a BPM config
type Volume struct {
	Path            string `yaml:"path,omitempty"`
	Writable        bool   `yaml:"writable,omitempty"`
	AllowExecutions bool   `yaml:"allow_executions,omitempty"`
	MountOnly       bool   `yaml:"mount_only,omitempty"`
}

// Unsafe from a BPM config
type Unsafe struct {
	Privileged          bool     `yaml:"privileged,omitempty"`
	UnrestrictedVolumes []Volume `yaml:"unrestricted_volumes,omitempty"`
}

// Process from a BPM config
type Process struct {
	Name              string            `yaml:"name,omitempty"`
	Executable        string            `yaml:"executable,omitempty"`
	Args              []string          `yaml:"args,omitempty"`
	Env               map[string]string `yaml:"env,omitempty"`
	Workdir           string            `yaml:"workdir,omitempty"`
	Hooks             Hooks             `yaml:"hooks,omitempty"`
	Capabilities      []string          `yaml:"capabilities,omitempty"`
	Limits            Limits            `yaml:"limits,omitempty"`
	EphemeralDisk     bool              `yaml:"ephemeral_disk,omitempty"`
	PersistentDisk    bool              `yaml:"persistent_disk,omitempty"`
	AdditionalVolumes []Volume          `yaml:"additional_volumes,omitempty"`
	Unsafe            Unsafe            `yaml:"unsafe,omitempty"`
}

// Config represent a BPM configuration
type Config struct {
	Processes []Process `yaml:"processes,omitempty"`
}

// NewConfig creates a new Config object from the yaml
func NewConfig(data []byte) (Config, error) {
	config := Config{}
	err := yaml.Unmarshal(data, &config)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}
