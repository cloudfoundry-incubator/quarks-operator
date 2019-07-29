package manifest

import (
	"fmt"
	"strings"
)

// JobProviderLinks provides links to other jobs, indexed by provider type and name
type JobProviderLinks map[string]map[string]JobLink

// Lookup returns a link for a type and name, used when links are consumed
func (jpl JobProviderLinks) Lookup(provider *JobSpecProvider) (JobLink, bool) {
	link, ok := jpl[provider.Type][provider.Name]
	return link, ok
}

// Add another job to the lookup map
func (jpl JobProviderLinks) Add(job Job, spec JobSpec, jobsInstances []JobInstance, linkAddress string) error {
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

		if providers, ok := jpl[linkType]; ok {
			if _, ok := providers[linkName]; ok {
				// If this comes from an addon, it will inevitably cause
				// conflicts. So in this case, we simply ignore the error
				if job.Properties.BOSHContainerization.IsAddon {
					continue
				}

				return fmt.Errorf("multiple providers for link: name=%s type=%s", linkName, linkType)
			}
		}

		if _, ok := jpl[linkType]; !ok {
			jpl[linkType] = map[string]JobLink{}
		}

		// construct the jobProviderLinks of the current job that provides
		// a link
		jpl[linkType][linkName] = JobLink{
			Address:    linkAddress,
			Instances:  jobsInstances,
			Properties: properties,
		}
	}
	return nil
}
