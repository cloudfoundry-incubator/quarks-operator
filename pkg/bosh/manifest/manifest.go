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
	Canaries        int
	MaxInFlight     string
	CanaryWatchTime string
	UpdateWatchTime string
	Serial          bool
	VMStratergy     string
}

// MigratedFrom from BOSH deployment manifest
type MigratedFrom struct {
	Name string
	Az   string
}

// IPv6 from BOSH deployment manifest
type IPv6 struct {
	Enable bool
}

// AgentEnvBoshConfig from BOSH deployment manifest
type AgentEnvBoshConfig struct {
	password              string
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
	Name               string                      `yaml:"name"`
	Instances          int                         `yaml:"instances"`
	Azs                []string                    `yaml:"azs"`
	Jobs               []Job                       `yaml:"jobs"`
	VMType             string                      `yaml:"vm_type"`
	VMExtensions       []string                    `yaml:"vm_extensions"`
	VMResources        VMResource                  `yaml:"vm_resources"`
	Stemcell           string                      `yaml:"stemcell"`
	PersistentDisk     int                         `yaml:"persistent_disk"`
	PersistentDiskType string                      `yaml:"persistent_disk_type"`
	Networks           []Network                   `yaml:"networks"`
	Update             Update                      `yaml:"update"`
	MigratedFrom       MigratedFrom                `yaml:"migrated_from"`
	LifeCycle          string                      `yaml:"lifecycle"`
	Properties         map[interface{}]interface{} `yaml:"properties"` // Doubt
	Env                AgentEnv                    `yaml:"env"`
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

// Manifest is a BOSH deployment manifest
type Manifest struct {
	InstanceGroups []InstanceGroup   `yaml:"instance-groups"`
	Features       Feature           `yaml:"features"`
	Variable       Variable          `yaml:"variables"`
	Tags           map[string]string `yaml:"tags"`
}
