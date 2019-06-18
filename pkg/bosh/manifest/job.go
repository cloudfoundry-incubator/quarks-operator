package manifest

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
)

// JobSpecFilename is the name of the job spec manifest in an unpacked BOSH release
const JobSpecFilename = "job.MF"

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

func (j *Job) dataDirs(name string) []string {
	return []string{
		filepath.Join(VolumeDataDirMountPath, name),
		filepath.Join(VolumeDataDirMountPath, "sys", "log", name),
		filepath.Join(VolumeDataDirMountPath, "sys", "run", name),
	}
}

func (j *Job) sysDirs(name string) []string {
	return []string{
		filepath.Join(VolumeSysDirMountPath, "log", name),
		filepath.Join(VolumeSysDirMountPath, "run", name),
	}
}

// JobProperties represents the properties map of a Job
type JobProperties struct {
	BOSHContainerization `yaml:"bosh_containerization"`
	Properties           map[string]interface{} `yaml:",inline"`
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
