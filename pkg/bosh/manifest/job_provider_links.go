package manifest

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// jobProviderLinks provides links to other jobs, indexed by provider type and name
type jobProviderLinks struct {
	links          map[string]map[string]JobLink
	instanceGroups map[string]map[string]JobLinkProperties
}

func newJobProviderLinks() jobProviderLinks {
	return jobProviderLinks{
		links:          map[string]map[string]JobLink{},
		instanceGroups: map[string]map[string]JobLinkProperties{},
	}
}

// Lookup returns a link for a type and name, used when links are consumed
func (jpl jobProviderLinks) Lookup(provider *JobSpecProvider) (JobLink, bool) {
	link, ok := jpl.links[provider.Type][provider.Name]
	return link, ok
}

// Add another job to the lookup maps
func (jpl jobProviderLinks) Add(igName string, job Job, spec JobSpec, jobsInstances []JobInstance, linkAddress string) error {
	var properties map[string]interface{}

	for _, link := range spec.Provides {
		properties = map[string]interface{}{}
		for _, property := range link.Properties {
			// generate a nested struct of map[string]interface{} when
			// a property is of the form foo.bar
			if strings.Contains(property, ".") {
				spec.RetrieveNestedProperty(properties, property)
			} else {
				properties[property] = spec.RetrievePropertyDefault(property)
			}
		}
		// Override default spec values with explicit settings from the
		// current bosh deployment manifest, this should be done under each
		// job, inside a `properties` key.
		for _, propertyName := range link.Properties {
			mergeNestedExplicitProperty(properties, job, propertyName)
		}
		linkName := link.Name
		linkType := link.Type

		// instance_group.job can override the link name through the
		// instance_group.job.provides, via the "as" key
		if job.Provides != nil {
			if value, ok := job.Provides[linkName]; ok {
				switch value := value.(type) {
				case map[string]interface{}:
					if overrideLinkName, ok := value["as"]; ok {
						linkName = fmt.Sprintf("%v", overrideLinkName)
					}
				case string:
					// As defined in the BOSH documentation, an explicit value of "nil" for
					// the provider means the link is "blocked"
					// https://bosh.io/docs/links/#blocking-link-provider
					if value == "nil" {
						continue
					}
					return fmt.Errorf("unexpected string detected: %v, can only be 'nil' to block the link", value)
				default:
					return fmt.Errorf("unexpected type detected: %T, should have been a map", value)
				}

			}
		}

		if providers, ok := jpl.links[linkType]; ok {
			if _, ok := providers[linkName]; ok {
				// If this comes from an addon, it will inevitably cause
				// conflicts. So in this case, we simply ignore the error
				if job.Properties.Quarks.IsAddon {
					continue
				}

				return fmt.Errorf("multiple providers for link: name=%s type=%s", linkName, linkType)
			}
		}

		if _, ok := jpl.links[linkType]; !ok {
			jpl.links[linkType] = map[string]JobLink{}
		}

		// construct the jobProviderLinks of the current job that provides
		// a link
		jpl.links[linkType][linkName] = JobLink{
			Address:    linkAddress,
			Instances:  jobsInstances,
			Properties: properties,
		}

		if _, ok := jpl.instanceGroups[igName]; !ok {
			jpl.instanceGroups[igName] = map[string]JobLinkProperties{}
		}
		jpl.instanceGroups[igName][names.EntanglementSecretKey(linkType, linkName)] = properties
	}
	return nil
}

// AddExternalLink adds link info from an external (non-BOSH) source
func (jpl jobProviderLinks) AddExternalLink(linkName string, linkType string, linkAddress string, jobsInstances []JobInstance, properties JobLinkProperties) {
	if _, ok := jpl.links[linkType]; !ok {
		jpl.links[linkType] = map[string]JobLink{}
	}

	jpl.links[linkType][linkName] = JobLink{
		Address:    linkAddress,
		Instances:  jobsInstances,
		Properties: properties,
	}
}
