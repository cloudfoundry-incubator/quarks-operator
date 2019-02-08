package manifest

// InstanceGroup from BOSH deployment manifest
type InstanceGroup struct {
	Name      string
	Instances int
	vm_type   string # Added by Svk Rohit
}

type Release struct {               # Added by Svk Rohit
	Name string
	Version float64 
}


// Manifest is a BOSH deployment manifest
type Manifest struct {
	InstanceGroups []InstanceGroup `yaml:"instance-groups"`
	Releases []Release `yaml:"releases"		# Added by Svk Rohit
}

