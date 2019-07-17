package manifest

import (
	"fmt"

	"github.com/pkg/errors"
)

type matcher func(*InstanceGroup, *AddOnPlacementRules) (bool, error)

// jobMatch matches stemcell rules for addon placement
func (m *Manifest) stemcellMatch(instanceGroup *InstanceGroup, rules *AddOnPlacementRules) (bool, error) {
	if instanceGroup == nil || rules == nil {
		return false, nil
	}

	osList := map[string]struct{}{}

	for _, job := range instanceGroup.Jobs {
		os, err := m.GetJobOS(instanceGroup.Name, job.Name)
		if err != nil {
			return false, errors.Wrap(err, "failed to calculate OS for BOSH job")
		}

		osList[os] = struct{}{}
	}

	for _, s := range rules.Stemcell {
		if _, osPresent := osList[s.OS]; osPresent {
			return true, nil
		}
	}

	return false, nil
}

// jobMatch matches job rules for addon placement
func (m *Manifest) jobMatch(instanceGroup *InstanceGroup, rules *AddOnPlacementRules) (bool, error) {
	if instanceGroup == nil || rules == nil {
		return false, nil
	}

	jobList := map[string]struct{}{}

	for _, job := range instanceGroup.Jobs {
		// We keep a map with keys release:job, so we can quickly determine later if
		// a job exists or not
		jobList[fmt.Sprintf("%s:%s", job.Release, job.Name)] = struct{}{}
	}

	for _, job := range rules.Jobs {
		if _, jobPresent := jobList[fmt.Sprintf("%s:%s", job.Release, job.Name)]; jobPresent {
			return true, nil
		}
	}

	return false, nil
}

// instanceGroupMatch matches instance group rules for addon placement
func (m *Manifest) instanceGroupMatch(instanceGroup *InstanceGroup, rules *AddOnPlacementRules) (bool, error) {
	if instanceGroup == nil || rules == nil {
		return false, nil
	}

	for _, ig := range rules.InstanceGroup {
		if ig == instanceGroup.Name {
			return true, nil
		}
	}

	return false, nil
}

// addOnPlacementMatch returns true if any placement rule of the addon matches the instance group
func (m *Manifest) addOnPlacementMatch(instanceGroup *InstanceGroup, rules *AddOnPlacementRules) (bool, error) {
	matchers := []matcher{
		m.stemcellMatch,
		m.jobMatch,
		m.instanceGroupMatch,
	}

	matchResult := false

	for _, matcher := range matchers {
		matched, err := matcher(instanceGroup, rules)
		if err != nil {
			return false, errors.Wrap(err, "failed to process match")
		}

		matchResult = matchResult || matched
	}

	return matchResult, nil
}
