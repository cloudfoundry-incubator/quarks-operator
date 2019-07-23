// Package manifest represents a valid BOSH manifest and provides funcs to load
// it, marshal it and access its fields.
package manifest

import (
	"crypto/sha1"
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

const (
	// DesiredManifestKeyName is the name of the key in desired manifest secret
	DesiredManifestKeyName = "manifest.yaml"
)

// Feature from BOSH deployment manifest
type Feature struct {
	ConvergeVariables    bool  `json:"converge_variables"`
	RandomizeAzPlacement *bool `json:"randomize_az_placement,omitempty"`
	UseDNSAddresses      *bool `json:"use_dns_addresses,omitempty"`
	UseTmpfsJobConfig    *bool `json:"use_tmpfs_job_config,omitempty"`
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
	CommonName       string     `json:"common_name"`
	AlternativeNames []string   `json:"alternative_names,omitempty"`
	IsCA             bool       `json:"is_ca"`
	CA               string     `json:"ca,omitempty"`
	ExtendedKeyUsage []AuthType `json:"extended_key_usage,omitempty"`
}

// Variable from BOSH deployment manifest
type Variable struct {
	Name    string           `json:"name"`
	Type    string           `json:"type"`
	Options *VariableOptions `json:"options,omitempty"`
}

// Stemcell from BOSH deployment manifest
type Stemcell struct {
	Alias   string `json:"alias"`
	OS      string `json:"os,omitempty"`
	Version string `json:"version"`
	Name    string `json:"name,omitempty"`
}

// ReleaseStemcell from BOSH deployment manifest
type ReleaseStemcell struct {
	OS      string `json:"os"`
	Version string `json:"version"`
}

// Release from BOSH deployment manifest
type Release struct {
	Name     string           `json:"name"`
	Version  string           `json:"version"`
	URL      string           `json:"url,omitempty"`
	SHA1     string           `json:"sha1,omitempty"`
	Stemcell *ReleaseStemcell `json:"stemcell,omitempty"`
}

// AddOnJob from BOSH deployment manifest
type AddOnJob struct {
	Name       string                 `json:"name"`
	Release    string                 `json:"release"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// AddOnStemcell from BOSH deployment manifest
type AddOnStemcell struct {
	OS string `json:"os"`
}

// AddOnPlacementJob from BOSH deployment manifest
type AddOnPlacementJob struct {
	Name    string `json:"name"`
	Release string `json:"release"`
}

// AddOnPlacementRules from BOSH deployment manifest
type AddOnPlacementRules struct {
	Stemcell      []*AddOnStemcell     `json:"stemcell,omitempty"`
	Deployments   []string             `json:"deployments,omitempty"`
	Jobs          []*AddOnPlacementJob `json:"release,omitempty"`
	InstanceGroup []string             `json:"instance_groups,omitempty"`
	Networks      []string             `json:"networks,omitempty"`
	Teams         []string             `json:"teams,omitempty"`
}

// AddOn from BOSH deployment manifest
type AddOn struct {
	Name    string               `json:"name"`
	Jobs    []AddOnJob           `json:"jobs"`
	Include *AddOnPlacementRules `json:"include,omitempty"`
	Exclude *AddOnPlacementRules `json:"exclude,omitempty"`
}

// Manifest is a BOSH deployment manifest
type Manifest struct {
	Name           string                   `json:"name"`
	DirectorUUID   string                   `json:"director_uuid"`
	InstanceGroups []*InstanceGroup         `json:"instance_groups,omitempty"`
	Features       *Feature                 `json:"features,omitempty"`
	Tags           map[string]string        `json:"tags,omitempty"`
	Releases       []*Release               `json:"releases,omitempty"`
	Stemcells      []*Stemcell              `json:"stemcells,omitempty"`
	AddOns         []*AddOn                 `json:"addons,omitempty"`
	Properties     []map[string]interface{} `json:"properties,omitempty"`
	Variables      []Variable               `json:"variables,omitempty"`
	Update         *Update                  `json:"update,omitempty"`
}

// LoadYAML returns a new BOSH deployment manifest from a yaml representation
func LoadYAML(data []byte) (*Manifest, error) {
	m := &Manifest{}
	err := yaml.Unmarshal(data, m)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal BOSH deployment manifest")
	}

	return m, nil
}

// Marshal serializes a BOSH manifest into yaml
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

// GetJobOS returns the stemcell layer OS used for a Job
// This is used for matching addon placement rules
func (m *Manifest) GetJobOS(instanceGroupName, jobName string) (string, error) {
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

			var stemcellOS string

			if release.Stemcell != nil {
				stemcellOS = release.Stemcell.OS
			} else {
				if stemcell == nil {
					return "", fmt.Errorf("stemcell OS could not be resolved for instance group %s", instanceGroup.Name)
				}
				stemcellOS = stemcell.OS
			}
			return stemcellOS, nil
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

// ImplicitVariables returns a list of all implicit variables in a manifest
func (m *Manifest) ImplicitVariables() ([]string, error) {
	varMap := make(map[string]bool)

	manifestBytes, err := m.Marshal()
	if err != nil {
		return nil, err
	}

	rawManifest := string(manifestBytes)

	// Collect all variables
	varRegexp := regexp.MustCompile(`\(\((!?[-/\.\w\pL]+)\)\)`)
	for _, match := range varRegexp.FindAllStringSubmatch(rawManifest, -1) {
		// Remove subfields from the match, e.g. ca.private_key -> ca
		fieldRegexp := regexp.MustCompile(`[^\.]+`)
		main := fieldRegexp.FindString(match[1])

		varMap[main] = true
	}

	// Remove the explicit ones
	for _, v := range m.Variables {
		varMap[v.Name] = false
	}

	names := []string{}
	for k, v := range varMap {
		if v {
			names = append(names, k)
		}
	}

	return names, nil
}

// ApplyAddons goes through all defined addons and adds jobs to matched instance groups
func (m *Manifest) ApplyAddons() error {
	for _, addon := range m.AddOns {
		for _, ig := range m.InstanceGroups {
			include, err := m.addOnPlacementMatch(ig, addon.Include)
			if err != nil {
				return errors.Wrap(err, "failed to process include placement matches")
			}
			exclude, err := m.addOnPlacementMatch(ig, addon.Exclude)
			if err != nil {
				return errors.Wrap(err, "failed to process exclude placement matches")
			}

			if exclude || !include {
				continue
			}

			for _, addonJob := range addon.Jobs {
				ig.Jobs = append(ig.Jobs, Job{
					Name:    addonJob.Name,
					Release: addonJob.Release,
					Properties: JobProperties{
						BOSHContainerization: BOSHContainerization{IsAddon: true},
						Properties:           addonJob.Properties,
					},
				})
			}
		}
	}

	// Remove addons after applying them, so we don't end up applying them again
	m.AddOns = nil

	return nil
}
