package manifest

import (
	"fmt"
	"regexp"
)

// ExtractWatchTime computes the watch time from a range or an absolute value
// This parses the time string used in the BOSH manifest's update config:
// https://bosh.io/docs/manifest-v2/#update
func ExtractWatchTime(rawWatchTime string) (string, error) {
	if rawWatchTime == "" {
		return "", nil
	}

	rangeRegex := regexp.MustCompile(`^\s*(\d+)\s*-\s*(\d+)\s*$`) // https://github.com/cloudfoundry/bosh/blob/914edca5278b994df7d91620c4f55f1c6665f81c/src/bosh-director/lib/bosh/director/deployment_plan/update_config.rb#L128
	if matches := rangeRegex.FindStringSubmatch(rawWatchTime); len(matches) > 0 {
		// Ignore the lower boundary, because the API-Server triggers reconciles
		return matches[2], nil
	}
	absoluteRegex := regexp.MustCompile(`^\s*(\d+)\s*$`) // https://github.com/cloudfoundry/bosh/blob/914edca5278b994df7d91620c4f55f1c6665f81c/src/bosh-director/lib/bosh/director/deployment_plan/update_config.rb#L130
	if matches := absoluteRegex.FindStringSubmatch(rawWatchTime); len(matches) > 0 {
		return matches[1], nil
	}
	return "", fmt.Errorf("watch time string did not match regexp: %s", rawWatchTime)
}
