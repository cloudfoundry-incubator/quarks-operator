package manifest

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	bc "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/containerization"
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
	Name       string                 `yaml:"name"`
	Release    string                 `yaml:"release"`
	Consumes   map[string]interface{} `yaml:"consumes,omitempty"`
	Provides   map[string]interface{} `yaml:"provides,omitempty"`
	Properties JobProperties          `yaml:"properties,omitempty"`
}

func (j *Job) specDir(baseDir string) string {
	return filepath.Join(baseDir, "jobs-src", j.Release, j.Name)
}

func (j *Job) loadSpec(baseDir string) (*JobSpec, error) {
	jobMFFilePath := filepath.Join(j.specDir(baseDir), JobSpecFilename)
	jobMfBytes, err := ioutil.ReadFile(jobMFFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read file")
	}

	jobSpec := JobSpec{}
	if err := yaml.Unmarshal([]byte(jobMfBytes), &jobSpec); err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal")
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

// JobProperties represents the properties map of a Job
type JobProperties struct {
	BOSHContainerization bc.BOSHContainerization `yaml:"bosh_containerization"`
	Properties           map[string]interface{}  `yaml:",inline"`
}

// ToMap returns a complete map with all properties, including the
// bosh_containerization key
func (p *JobProperties) ToMap() map[string]interface{} {
	result := map[string]interface{}{}

	for k, v := range p.Properties {
		result[k] = v
	}

	result["bosh_containerization"] = p.BOSHContainerization

	return result
}
