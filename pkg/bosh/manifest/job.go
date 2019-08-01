package manifest

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

const (
	// DataDir the mount path for the data directory.
	DataDir = "/var/vcap/data"

	// SysDir the mount path for the sys directory.
	SysDir = "/var/vcap/sys"

	// JobSpecFilename is the name of the job spec manifest in an unpacked BOSH release
	JobSpecFilename = "job.MF"
)

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
	Consumes []JobSpecProvider
	Provides []JobSpecLink
}

// JobSpecProvider represents a provider in the job spec Consumes field.
type JobSpecProvider struct {
	Name     string
	Type     string
	Optional bool
}

// JobSpecLink represents a link in the job spec Provides field.
type JobSpecLink struct {
	Name       string
	Type       string
	Properties []string
}

// Job from BOSH deployment manifest
type Job struct {
	Name       string                 `json:"name"`
	Release    string                 `json:"release"`
	Consumes   map[string]interface{} `json:"consumes,omitempty"`
	Provides   map[string]interface{} `json:"provides,omitempty"`
	Properties JobProperties          `json:"properties,omitempty"`
}

// JobProperties represents the properties map of a Job
type JobProperties struct {
	Quarks     Quarks                 `json:"quarks"`
	Properties map[string]interface{} `json:"-"`
}

// MarshalJSON is implemented to support inlining Properties
func (p *JobProperties) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.ToMap())
}

// UnmarshalJSON is implemented to support inlining properties
func (p *JobProperties) UnmarshalJSON(b []byte) error {
	var j map[string]interface{}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	err := d.Decode(&j)
	if err != nil {
		return err
	}

	return p.FromMap(j)
}

func (j *Job) specDir(baseDir string) string {
	return filepath.Join(baseDir, "jobs-src", j.Release, j.Name)
}

func (j *Job) loadSpec(baseDir string) (*JobSpec, error) {
	jobMFFilePath := filepath.Join(j.specDir(baseDir), JobSpecFilename)
	jobMfBytes, err := ioutil.ReadFile(jobMFFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file in path %s", jobMFFilePath)
	}

	jobSpec := JobSpec{}
	// Make sure to use json.Number when unmarshaling
	if err := yaml.Unmarshal([]byte(jobMfBytes), &jobSpec, func(d *json.Decoder) *json.Decoder {
		d.UseNumber()
		return d
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal file %s", jobMFFilePath)
	}

	return &jobSpec, nil
}

// DataDirs returns all data dirs a BOSH job expects
func (j *Job) DataDirs() []string {
	return []string{
		filepath.Join(DataDir, j.Name),
		filepath.Join(DataDir, "sys", "log", j.Name),
		filepath.Join(DataDir, "sys", "run", j.Name),
	}
}

// SysDirs returns all sys dirs a BOSH job expects
func (j *Job) SysDirs() []string {
	return []string{
		filepath.Join(SysDir, "log", j.Name),
		filepath.Join(SysDir, "run", j.Name),
	}
}

// ToMap returns a complete map with all properties, including the
// quarks key
func (p *JobProperties) ToMap() map[string]interface{} {
	result := map[string]interface{}{}

	for k, v := range p.Properties {
		result[k] = v
	}

	result["quarks"] = p.Quarks

	return result
}

// FromMap populates a JobProperties based on a map
func (p *JobProperties) FromMap(properties map[string]interface{}) error {
	// **Important** We have to use the sigs yaml parser here - this
	// struct contains kube objects
	quarksBytes, err := yaml.Marshal(properties["quarks"])
	if err != nil {
		return err
	}

	quarks := Quarks{}
	err = yaml.Unmarshal(quarksBytes, &quarks, func(opt *json.Decoder) *json.Decoder {
		opt.UseNumber()
		return opt
	})
	if err != nil {
		return err
	}

	p.Properties = properties
	delete(p.Properties, "quarks")
	p.Quarks = quarks

	return nil
}
