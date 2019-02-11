package manifest

// Job from BOSH deployment manifest
type Job struct {
	Name       string
	Release    string
	Consumes   map[string]string
	Provides   map[string]string
	Properties map[string]string
}

// VMResource from BOSH deployment manifest
type VMResource struct {
	CPU               int
	RAM               int
	EphemeralDiskSize int
}

// Network from BOSH deployment manifest
type Network struct {
	Name      string
	StaticIps []string
	Default   []string
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
	Enable bool
}

// AgentEnvBoshConfig from BOSH deployment manifest
type AgentEnvBoshConfig struct {
	Password              string
	KeepRootPassword      string
	RemoveDevTools        bool
	RemoveStaticLibraries bool
	SwapSize              int
	IPv6                  IPv6
}

// AgentEnv from BOSH deployment manifest
type AgentEnv struct {
	PersistentDiskFS           string   `yaml:"name"`
	PersistentDiskMountOptions []string `yaml:"name"`
	AgentEnvBoshConfig         []string `yaml:"name"`
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
	Properties         map[string]interface{} `yaml:"properties"` // Doubt
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
	InstanceGroups []InstanceGroup        `yaml:"instance-groups"`
	Features       Feature                `yaml:"features"`
	Variable       Variable               `yaml:"variables"`
	Tags           map[string]string      `yaml:"tags"`
	Name           string                 `yaml:"name"`
	DirectorUUID   string                 `yaml:"director_uuid"`
	Releases       []Release              `yaml:"releases"`
	Stemcells      []Stemcell             `yaml:"stemcells"`
	AddOns         []AddOn                `yaml:"addons"`
	Properties     map[string]interface{} `yaml:"properties"`
	Variables      []Variable             `yaml:"variables"`
}
