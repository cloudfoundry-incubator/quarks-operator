package names

import (
	"fmt"
	"strings"

	sharednames "code.cloudfoundry.org/quarks-utils/pkg/names"
)

// QuarksLinkSecretName returns the name of a secret used for Quarks links
// to be mounted or used by environment variables
// `link-<suffix>-<suffix>...`
func QuarksLinkSecretName(suffixes ...string) string {
	return sharednames.SanitizeSubdomain(strings.Join(
		append([]string{"link"}, suffixes...),
		"-",
	))
}

// QuarksLinkSecretKey returns the key (composed of type and name), which is
// used as the root level key for the data mapping in secrets
func QuarksLinkSecretKey(linkType, linkName string) string {
	return fmt.Sprintf("%s-%s", linkType, linkName)
}
