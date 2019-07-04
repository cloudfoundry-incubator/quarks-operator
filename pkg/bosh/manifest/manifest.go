// Package manifest represents a valid BOSH manifest and provides funcs to load
// it, marshal it and access its fields.
package manifest

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	bc "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/containerization"
)

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

// LoadYAML returns a new BOSH deployment manifest from a yaml representation
func LoadYAML(data []byte) (*Manifest, error) {
	m := &Manifest{}
	err := yaml.Unmarshal(data, m)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal BOSH deployment manifest")
	}

	bcm, err := bc.LoadKubeYAML(data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal BOSHContainerization from deployment manifest")
	}

	for i, ig := range bcm.InstanceGroups {
		for j, job := range ig.Jobs {
			m.InstanceGroups[i].Jobs[j].Properties.BOSHContainerization = job.Properties.BOSHContainerization
		}
		m.InstanceGroups[i].Env.AgentEnvBoshConfig.Agent.Settings.Affinity = ig.Env.BOSH.Agent.Settings.Affinity
	}

	return m, nil
}

func (m *Manifest) Marshal() ([]byte, error) {
	return yaml.Marshal(m)
}

// SHA1 calculates the SHA1 of the manifest
func (m *Manifest) SHA1() (string, error) {
	manifestBytes, err := m.Marshal()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha1.Sum(manifestBytes)), nil
}

// GetReleaseImage returns the release image location for a given instance group/job
func (m *Manifest) GetReleaseImage(instanceGroupName, jobName string) (string, error) {
	var instanceGroup *InstanceGroup
	for i := range m.InstanceGroups {
		if m.InstanceGroups[i].Name == instanceGroupName {
			instanceGroup = m.InstanceGroups[i]
			break
		}
	}
	if instanceGroup == nil {
		return "", fmt.Errorf("instance group '%s' not found", instanceGroupName)
	}

	var stemcell *Stemcell
	for i := range m.Stemcells {
		if m.Stemcells[i].Alias == instanceGroup.Stemcell {
			stemcell = m.Stemcells[i]
		}
	}

	var job *Job
	for i := range instanceGroup.Jobs {
		if instanceGroup.Jobs[i].Name == jobName {
			job = &instanceGroup.Jobs[i]
			break
		}
	}
	if job == nil {
		return "", fmt.Errorf("job '%s' not found in instance group '%s'", jobName, instanceGroupName)
	}

	for i := range m.Releases {
		if m.Releases[i].Name == job.Release {
			release := m.Releases[i]
			name := strings.TrimRight(release.URL, "/")

			var stemcellVersion string

			if release.Stemcell != nil {
				stemcellVersion = release.Stemcell.OS + "-" + release.Stemcell.Version
			} else {
				if stemcell == nil {
					return "", fmt.Errorf("stemcell could not be resolved for instance group %s", instanceGroup.Name)
				}
				stemcellVersion = stemcell.OS + "-" + stemcell.Version
			}
			return fmt.Sprintf("%s/%s:%s-%s", name, release.Name, stemcellVersion, release.Version), nil
		}
	}
	return "", fmt.Errorf("release '%s' not found", job.Release)
}

// InstanceGroupByName returns the instance group identified by the given name
func (m *Manifest) InstanceGroupByName(name string) (*InstanceGroup, error) {
	for _, instanceGroup := range m.InstanceGroups {
		if instanceGroup.Name == name {
			return instanceGroup, nil
		}
	}

	return nil, errors.Errorf("can't find instance group '%s' in manifest", name)
}
