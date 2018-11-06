package manifest

// InstanceGroup from BOSH deployment manifest
type InstanceGroup struct {
	Name      string
	Instances int
}

// Manifest is a BOSH deployment manifest
type Manifest struct {
	InstanceGroups []InstanceGroup `yaml:"instance-groups"`
}
