package manifest

import (
	"fmt"

	"github.com/pkg/errors"
)

func (m *Manifest) lookupJobInInstanceGroup(igName, boshJobName string) (*Job, error) {
	instanceGroup, err := m.lookupInstanceGroup(igName)
	if err != nil {
		return nil, errors.Wrap(err, "job not found")
	}

	for _, job := range instanceGroup.Jobs {
		if job.Name == boshJobName {
			return &job, nil
		}
	}

	return nil, fmt.Errorf("job %s not found", boshJobName)
}

func (m *Manifest) lookupInstanceGroup(igName string) (*InstanceGroup, error) {
	for _, instanceGroup := range m.InstanceGroups {
		if instanceGroup.Name == igName {
			return instanceGroup, nil
		}
	}

	return nil, fmt.Errorf("instance group %s not found", igName)

}
