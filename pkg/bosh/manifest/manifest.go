package manifest

import "github.com/pkg/errors"

// Link with name for rendering
type Link struct {
	Name       string        `yaml:"name"`
	Instances  []JobInstance `yaml:"instances"`
	Properties interface{}   `yaml:"properties"`
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

// InstanceGroupByName returns the instance group identified by the given name
func (m Manifest) InstanceGroupByName(name string) (*InstanceGroup, error) {
	for _, instanceGroup := range m.InstanceGroups {
		if instanceGroup.Name == name {
			return instanceGroup, nil
		}
	}

	return nil, errors.Errorf("can't find instance group '%s' in manifest", name)
}
