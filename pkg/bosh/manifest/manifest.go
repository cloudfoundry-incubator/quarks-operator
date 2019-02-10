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
	PersistentDiskFS           string
	PersistentDiskMountOptions []string
	AgentEnvBoshConfig         []string
}

// InstanceGroup from BOSH deployment manifest
type InstanceGroup struct {
	Name               string
	Instances          int
	Azs                []string
	Jobs               []Job
	VMType             string
	VMExtensions       []string
	VMResources        VMResource
	Stemcell           string
	PersistentDisk     int
	PersistentDiskType string
	Networks           []Network
	Update             Update
	MigratedFrom       MigratedFrom
	LifeCycle          string
	Properties         map[string]string
	Env                AgentEnv
}

// Manifest is a BOSH deployment manifest
type Manifest struct {
	InstanceGroups []InstanceGroup `yaml:"instance-groups"`
}
