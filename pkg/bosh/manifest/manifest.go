package manifest

// Job from BOSH deployment manifest
type Job struct {
	Name       string                 `yaml:"name"`
	Release    string                 `yaml:"release"`
	Consumes   map[string]interface{} `yaml:"consumes"`
	Provides   map[string]interface{} `yaml:"provides"`
	Properties map[string]interface{} `yaml:"properties"`
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
	StaticIps []string `yaml:"static_ips"`
	Default   []string `yaml:"default"`
}

// Update from BOSH deployment manifest
type Update struct {
	Canaries        int    `yaml:"canaries"`
	MaxInFlight     string `yaml:"max_in_flight"`
	CanaryWatchTime string `yaml:"canary_watch_time"`
	UpdateWatchTime string `yaml:"update_watch_time"`
	Serial          bool   `yaml:"serial"`
	VMStratergy     string `yaml:"vm_stratergy"`
}

// MigratedFrom from BOSH deployment manifest
type MigratedFrom struct {
	Name string `yaml:"name"`
	Az   string `yaml:"az"`
}

// IPv6 from BOSH deployment manifest
type IPv6 struct {
	Enable bool `yaml:"enable"`
}

// JobDir from BOSH deployment manifest
type JobDir struct {
	Tmpfs     bool   `yaml:"tmpfs"`
	TmpfsSize string `yaml:"tmpfs_size"`
}

// Agent from BOSH deployment manifest
type Agent struct {
	Settings string `yaml:"settings"`
	Tmpfs    bool   `yaml:"tmpfs"`
}

// AgentEnvBoshConfig from BOSH deployment manifest
type AgentEnvBoshConfig struct {
	Password              string `yaml:"password"`
	KeepRootPassword      string `yaml:"keep_root_password"`
	RemoveDevTools        bool   `yaml:"remove_dev_tools"`
	RemoveStaticLibraries bool   `yaml:"remove_static_libraries"`
	SwapSize              int    `yaml:"swap_size"`
	IPv6                  IPv6   `yaml:"ipv6"`
	JobDir                JobDir `yaml:"job_dir"`
	Agent                 Agent  `yaml:"agent"`
}

// AgentEnv from BOSH deployment manifest
type AgentEnv struct {
	PersistentDiskFS           string             `yaml:"persistent_disk_fs"`
	PersistentDiskMountOptions []string           `yaml:"persistent_disk_mount_options"`
	AgentEnvBoshConfig         AgentEnvBoshConfig `yaml:"bosh"`
}

// InstanceGroup from BOSH deployment manifest
type InstanceGroup struct {
	Name               string                 `yaml:"name"`
	Instances          int                    `yaml:"instances"`
	Azs                []string               `yaml:"azs"`
	Jobs               []Job                  `yaml:"jobs"`
	VMType             string                 `yaml:"vm_type"`
	VMExtensions       []string               `yaml:"vm_extensions"`
	VMResources        VMResource             `yaml:"vm_resources"`
	Stemcell           string                 `yaml:"stemcell"`
	PersistentDisk     int                    `yaml:"persistent_disk"`
	PersistentDiskType string                 `yaml:"persistent_disk_type"`
	Networks           []Network              `yaml:"networks"`
	Update             Update                 `yaml:"update"`
	MigratedFrom       MigratedFrom           `yaml:"migrated_from"`
	LifeCycle          string                 `yaml:"lifecycle"`
	Properties         map[string]interface{} `yaml:"properties"`
	Env                AgentEnv               `yaml:"env"`
}

// Feature from BOSH deployment manifest
type Feature struct {
	ConvergeVariables    bool `yaml:"name"`
	RandomizeAzPlacement bool `yaml:"randomize_az_placement"`
	UseDNSAddresses      bool `yaml:"use_dns_addresses"`
}

// Variable from BOSH deployment manifest
type Variable struct {
	Name    string            `yaml:"name"`
	Type    string            `yaml:"type"`
	Options map[string]string `yaml:"options"`
}

// Stemcell from BOSH deployment manifest
type Stemcell struct {
	Alias   string `yaml:"alias"`
	OS      string `yaml:"os"`
	Version string `yaml:"version"`
	Name    string `yaml:"name"`
}

// Release from BOSH deployment manifest
type Release struct {
	Name     string   `yaml:"name"`
	Version  string   `yaml:"version"`
	URL      string   `yaml:"url"`
	SHA1     string   `yaml:"sha1"`
	Stemcell Stemcell `yaml:"stemcell"`
}

// AddOnJob from BOSH deployment manifest
type AddOnJob struct {
	Name       string                 `yaml:"name"`
	Release    string                 `yaml:"release"`
	Properties map[string]interface{} `yaml:"properties"`
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
	Stemcell      []AddOnStemcell     `yaml:"stemcell"`
	Deployments   []string            `yaml:"deployments"`
	Jobs          []AddOnPlacementJob `yaml:"release"`
	InstanceGroup []string            `yaml:"instance_groups"`
	Networks      []string            `yaml:"networks"`
	Teams         []string            `yaml:"teams"`
}

// AddOn from BOSH deployment manifest
type AddOn struct {
	Name    string              `yaml:"name"`
	Jobs    []AddOnJob          `yaml:"jobs"`
	Include AddOnPlacementRules `yaml:"include"`
	Exclude AddOnPlacementRules `yaml:"exclude"`
}

// Manifest is a BOSH deployment manifest
type Manifest struct {
	Name           string                   `yaml:"name"`
	DirectorUUID   string                   `yaml:"director_uuid"`
	InstanceGroups []InstanceGroup          `yaml:"instance-groups"`
	Features       Feature                  `yaml:"features"`
	Tags           map[string]string        `yaml:"tags"`
	Releases       []Release                `yaml:"releases"`
	Stemcells      []Stemcell               `yaml:"stemcells"`
	AddOns         []AddOn                  `yaml:"addons"`
	Properties     []map[string]interface{} `yaml:"properties"`
	Variables      []Variable               `yaml:"variables"`
	Update         Update                   `yaml:"update"`
}
