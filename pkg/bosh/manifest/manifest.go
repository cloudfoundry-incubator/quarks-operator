package manifest

import (
	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// JobInstance for data gathering
type JobInstance struct {
	Address     string                 `yaml:"address"`
	AZ          string                 `yaml:"az"`
	ID          string                 `yaml:"id"`
	Index       int                    `yaml:"index"`
	Instance    int                    `yaml:"instance"`
	Name        string                 `yaml:"name"`
	BPM         bpm.Config             `yaml:"bpm"`
	Fingerprint interface{}            `yaml:"fingerprint"`
	Network     map[string]interface{} `yaml:"networks"`
	IP          string                 `yaml:"ip"`
}

// Link with name for rendering
type Link struct {
	Name       string        `yaml:"name"`
	Instances  []JobInstance `yaml:"instances"`
	Properties interface{}   `yaml:"properties"`
}

// JobLink describes links inside a job properties
// bosh_containerization.
type JobLink struct {
	Instances  []JobInstance          `yaml:"instances"`
	Properties map[string]interface{} `yaml:"properties"`
}

// JobSpec describes the contents of "job.MF" files
type JobSpec struct {
	Name        string
	Description string
	Packages    []string
	Templates   map[string]string
	Properties  map[string]struct {
		Description string
		Default     interface{}
		Example     interface{}
	}
	Consumes []struct {
		Name     string
		Type     string
		Optional bool
	}
	Provides []struct {
		Name       string
		Type       string
		Properties []string
	}
}

// Job from BOSH deployment manifest
type Job struct {
	Name       string                 `yaml:"name"`
	Release    string                 `yaml:"release"`
	Consumes   map[string]interface{} `yaml:"consumes,omitempty"`
	Provides   map[string]interface{} `yaml:"provides,omitempty"`
	Properties JobProperties          `yaml:"properties,omitempty"`
}

// BOSHContainerization represents the special 'bosh_containerization'
// property key
type BOSHContainerization struct {
	Consumes  map[string]JobLink `yaml:"consumes"`
	Instances []JobInstance      `yaml:"instances"`
	Release   string             `yaml:"release"`
	BPM       bpm.Config         `yaml:"bpm"`
}

// JobProperties represents the properties map of a Job
type JobProperties struct {
	BOSHContainerization `yaml:"bosh_containerization"`
	Properties           map[string]interface{} `yaml:",inline"`
}

// ToMap returns a complete map with all properties, including the
// bosh_containerization key
func (p *JobProperties) ToMap() map[string]interface{} {
	result := map[string]interface{}{}

	for k, v := range p.Properties {
		result[k] = v
	}

	result["bosh_containerization"] = p.BOSHContainerization

	return result
}

// VMResource from BOSH deployment manifest
type VMResource struct {
	CPU               int `yaml:"cpu"`
	RAM               int `yaml:"ram"`
	EphemeralDiskSize int `yaml:"ephemeral_disk_size"`
}

// Network from BOSH deployment manifest
type Network struct {
	Name      string   `yaml:"name"`
	StaticIps []string `yaml:"static_ips,omitempty"`
	Default   []string `yaml:"default,omitempty"`
}

// Update from BOSH deployment manifest
type Update struct {
	Canaries        int     `yaml:"canaries"`
	MaxInFlight     string  `yaml:"max_in_flight"`
	CanaryWatchTime string  `yaml:"canary_watch_time"`
	UpdateWatchTime string  `yaml:"update_watch_time"`
	Serial          bool    `yaml:"serial,omitempty"`
	VMStrategy      *string `yaml:"vm_strategy,omitempty"`
}

// MigratedFrom from BOSH deployment manifest
type MigratedFrom struct {
	Name string `yaml:"name"`
	Az   string `yaml:"az,omitempty"`
}

// IPv6 from BOSH deployment manifest
type IPv6 struct {
	Enable bool `yaml:"enable"`
}

// JobDir from BOSH deployment manifest
type JobDir struct {
	Tmpfs     *bool  `yaml:"tmpfs,omitempty"`
	TmpfsSize string `yaml:"tmpfs_size,omitempty"`
}

// Agent from BOSH deployment manifest
type Agent struct {
	Settings string `yaml:"settings,omitempty"`
	Tmpfs    *bool  `yaml:"tmpfs,omitempty"`
}

// AgentEnvBoshConfig from BOSH deployment manifest
type AgentEnvBoshConfig struct {
	Password              string  `yaml:"password,omitempty"`
	KeepRootPassword      string  `yaml:"keep_root_password,omitempty"`
	RemoveDevTools        *bool   `yaml:"remove_dev_tools,omitempty"`
	RemoveStaticLibraries *bool   `yaml:"remove_static_libraries,omitempty"`
	SwapSize              *int    `yaml:"swap_size,omitempty"`
	IPv6                  IPv6    `yaml:"ipv6,omitempty"`
	JobDir                *JobDir `yaml:"job_dir,omitempty"`
	Agent                 *Agent  `yaml:"agent,omitempty"`
}

// AgentEnv from BOSH deployment manifest
type AgentEnv struct {
	PersistentDiskFS           string              `yaml:"persistent_disk_fs,omitempty"`
	PersistentDiskMountOptions []string            `yaml:"persistent_disk_mount_options,omitempty"`
	AgentEnvBoshConfig         *AgentEnvBoshConfig `yaml:"bosh,omitempty"`
}

// InstanceGroup from BOSH deployment manifest
type InstanceGroup struct {
	Name               string                 `yaml:"name"`
	Instances          int                    `yaml:"instances"`
	Azs                []string               `yaml:"azs"`
	Jobs               []Job                  `yaml:"jobs"`
	VMType             string                 `yaml:"vm_type,omitempty"`
	VMExtensions       []string               `yaml:"vm_extensions,omitempty"`
	VMResources        *VMResource            `yaml:"vm_resources"`
	Stemcell           string                 `yaml:"stemcell"`
	PersistentDisk     *int                   `yaml:"persistent_disk,omitempty"`
	PersistentDiskType string                 `yaml:"persistent_disk_type,omitempty"`
	Networks           []*Network             `yaml:"networks,omitempty"`
	Update             *Update                `yaml:"update,omitempty"`
	MigratedFrom       *MigratedFrom          `yaml:"migrated_from,omitempty"`
	LifeCycle          string                 `yaml:"lifecycle,omitempty"`
	Properties         map[string]interface{} `yaml:"properties,omitempty"`
	Env                *AgentEnv              `yaml:"env,omitempty"`
}

// Feature from BOSH deployment manifest
type Feature struct {
	ConvergeVariables    bool  `yaml:"converge_variables"`
	RandomizeAzPlacement *bool `yaml:"randomize_az_placement,omitempty"`
	UseDNSAddresses      *bool `yaml:"use_dns_addresses,omitempty"`
	UseTmpfsJobConfig    *bool `yaml:"use_tmpfs_job_config,omitempty"`
}

// AuthType from BOSH deployment manifest
type AuthType string

// AuthType values from BOSH deployment manifest
const (
	ClientAuth AuthType = "client_auth"
	ServerAuth AuthType = "server_auth"
)

// VariableOptions from BOSH deployment manifest
type VariableOptions struct {
	CommonName       string     `yaml:"common_name"`
	AlternativeNames []string   `yaml:"alternative_names,omitempty"`
	IsCA             bool       `yaml:"is_ca"`
	CA               string     `yaml:"ca,omitempty"`
	ExtendedKeyUsage []AuthType `yaml:"extended_key_usage,omitempty"`
}

// Variable from BOSH deployment manifest
type Variable struct {
	Name    string           `yaml:"name"`
	Type    string           `yaml:"type"`
	Options *VariableOptions `yaml:"options,omitempty"`
}

// Stemcell from BOSH deployment manifest
type Stemcell struct {
	Alias   string `yaml:"alias"`
	OS      string `yaml:"os,omitempty"`
	Version string `yaml:"version"`
	Name    string `yaml:"name,omitempty"`
}

// ReleaseStemcell from BOSH deployment manifest
type ReleaseStemcell struct {
	OS      string `yaml:"os"`
	Version string `yaml:"version"`
}

// Release from BOSH deployment manifest
type Release struct {
	Name     string           `yaml:"name"`
	Version  string           `yaml:"version"`
	URL      string           `yaml:"url,omitempty"`
	SHA1     string           `yaml:"sha1,omitempty"`
	Stemcell *ReleaseStemcell `yaml:"stemcell,omitempty"`
}

// AddOnJob from BOSH deployment manifest
type AddOnJob struct {
	Name       string                 `yaml:"name"`
	Release    string                 `yaml:"release"`
	Properties map[string]interface{} `yaml:"properties,omitempty"`
}

// AddOnStemcell from BOSH deployment manifest
type AddOnStemcell struct {
	OS string `yaml:"os"`
}

// AddOnPlacementJob from BOSH deployment manifest
type AddOnPlacementJob struct {
	Name    string `yaml:"name"`
	Release string `yaml:"release"`
}

// AddOnPlacementRules from BOSH deployment manifest
type AddOnPlacementRules struct {
	Stemcell      []*AddOnStemcell     `yaml:"stemcell,omitempty"`
	Deployments   []string             `yaml:"deployments,omitempty"`
	Jobs          []*AddOnPlacementJob `yaml:"release,omitempty"`
	InstanceGroup []string             `yaml:"instance_groups,omitempty"`
	Networks      []string             `yaml:"networks,omitempty"`
	Teams         []string             `yaml:"teams,omitempty"`
}

// AddOn from BOSH deployment manifest
type AddOn struct {
	Name    string               `yaml:"name"`
	Jobs    []AddOnJob           `yaml:"jobs"`
	Include *AddOnPlacementRules `yaml:"include,omitempty"`
	Exclude *AddOnPlacementRules `yaml:"exclude,omitempty"`
}

// Manifest is a BOSH deployment manifest
type Manifest struct {
	Name           string                   `yaml:"name"`
	DirectorUUID   string                   `yaml:"director_uuid"`
	InstanceGroups []*InstanceGroup         `yaml:"instance_groups,omitempty"`
	Features       *Feature                 `yaml:"features,omitempty"`
	Tags           map[string]string        `yaml:"tags,omitempty"`
	Releases       []*Release               `yaml:"releases,omitempty"`
	Stemcells      []*Stemcell              `yaml:"stemcells,omitempty"`
	AddOns         []*AddOn                 `yaml:"addons,omitempty"`
	Properties     []map[string]interface{} `yaml:"properties,omitempty"`
	Variables      []Variable               `yaml:"variables,omitempty"`
	Update         *Update                  `yaml:"update,omitempty"`
}
