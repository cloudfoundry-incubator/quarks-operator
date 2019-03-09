package cmd

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	btg "github.com/viovanov/bosh-template-go"
	yaml "gopkg.in/yaml.v2"
)

// dataGatherCmd represents the dataGather command
var dataGatherCmd = &cobra.Command{
	Use:   "data-gather [flags]",
	Short: "Gathers data of a bosh manifest",
	Long: `Gathers data of a manifest.

This will retrieve information of an instance-group
inside a bosh manifest.

`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mFile := viper.GetString("gmanifest")
		if len(mFile) == 0 {
			return fmt.Errorf("manifest cannot be empty")
		}

		baseDir := viper.GetString("base_dir")
		if len(baseDir) == 0 {
			return fmt.Errorf("base directory cannot be empty")
		}

		ns := viper.GetString("desired_namespace")
		if len(ns) == 0 {
			return fmt.Errorf("namespace cannot be empty")
		}

		desiredIgs := viper.GetStringSlice("instance_groups")

		mBytes, err := ioutil.ReadFile(mFile)
		if err != nil {
			return err
		}

		mStruct := manifest.Manifest{}
		err = yaml.Unmarshal(mBytes, &mStruct)
		if err != nil {
			return err
		}

		return GatherData(&mStruct, baseDir, ns, desiredIgs)
	},
}

func init() {
	rootCmd.AddCommand(dataGatherCmd)

	dataGatherCmd.Flags().StringP("manifest", "m", "", "path to a bosh manifest")
	dataGatherCmd.Flags().String("desired-namespace", "", "the kubernetes namespace") //TODO: can we reuse the global ns flag
	dataGatherCmd.Flags().StringP("base-dir", "b", "", "a path to the base directory")
	dataGatherCmd.Flags().StringSliceP("instance-groups", "g", []string{}, "the instance-groups filter")

	// This will get the values from any set ENV var, but always
	// the values provided via the flags have more precedence.
	viper.AutomaticEnv()
	viper.BindPFlag("gmanifest", dataGatherCmd.Flags().Lookup("manifest"))
	viper.BindPFlag("desired_namespace", dataGatherCmd.Flags().Lookup("desired-namespace"))
	viper.BindPFlag("base_dir", dataGatherCmd.Flags().Lookup("base-dir"))
	viper.BindPFlag("instance_groups", dataGatherCmd.Flags().Lookup("instance-groups"))
}

// CollectReleaseSpecsAndProviderLinks will collect all release specs and bosh links for provider jobs
func CollectReleaseSpecsAndProviderLinks(mStruct *manifest.Manifest, baseDir string, ns string, desiredIgs []string) (map[string]map[string]manifest.JobSpec, map[string]map[string]manifest.JobLink, error) {
	// Contains YAML.load('.../release_name/job_name/job.MF')
	releaseSpecs := map[string]map[string]manifest.JobSpec{}

	// Lists every link provided by this manifest
	providerLinks := map[string]map[string]manifest.JobLink{}

	for igID, ig := range mStruct.InstanceGroups {
		// Filter based on the list passed via --instance-groups flag
		if len(desiredIgs) > 0 && !contains(desiredIgs, ig.Name) {
			continue
		}

		for idJob, job := range ig.Jobs {
			// make sure a map entry exists for the current job release
			if _, ok := releaseSpecs[job.Release]; !ok {
				releaseSpecs[job.Release] = map[string]manifest.JobSpec{}
			}

			// load job.MF into releaseSpecs[job.Release][job.Name]
			if _, ok := releaseSpecs[job.Release][job.Name]; !ok {
				jobMFFilePath := filepath.Join(baseDir, "jobs-src", job.Release, job.Name, "job.MF")
				jobMfBytes, err := ioutil.ReadFile(jobMFFilePath)
				if err != nil {
					return nil, nil, err
				}

				jobSpec := manifest.JobSpec{}
				if err := yaml.Unmarshal([]byte(jobMfBytes), &jobSpec); err != nil {
					return nil, nil, err
				}
				releaseSpecs[job.Release][job.Name] = jobSpec
			}

			// spec of the current jobs release/name
			spec := releaseSpecs[job.Release][job.Name]

			// Generate instance spec for each ig instance
			var jobsInstances []manifest.JobInstance
			for i := 0; i < ig.Instances; i++ {
				for _, az := range ig.Azs {
					index := len(jobsInstances)
					name := fmt.Sprintf("%s-%s", ig.Name, job.Name)
					id := fmt.Sprintf("%v-%v-%v", ig.Name, index, job.Name)
					address := fmt.Sprintf("%s.%s.svc.cluster.local", id, ns)

					jobsInstances = append(jobsInstances, manifest.JobInstance{
						Address:  address,
						AZ:       az,
						ID:       id,
						Index:    index,
						Instance: i,
						Name:     name,
					})
				}
			}

			// add bosh_containerization to properties
			if mStruct.InstanceGroups[igID].Jobs[idJob].Properties == nil {
				mStruct.InstanceGroups[igID].Jobs[idJob].Properties = map[string]interface{}{}
			}
			if _, ok := mStruct.InstanceGroups[igID].Jobs[idJob].Properties["bosh_containerization"]; !ok {
				mStruct.InstanceGroups[igID].Jobs[idJob].Properties["bosh_containerization"] = map[string]interface{}{}
			}
			mStruct.InstanceGroups[igID].Jobs[idJob].Properties["bosh_containerization"].(map[string]interface{})["instances"] = jobsInstances

			// Create a list of fully evaluated links provided by the current job
			if spec.Provides != nil {
				var properties map[string]interface{}

				for _, provider := range spec.Provides {
					properties = map[string]interface{}{}
					for _, property := range provider.Properties {
						// generate a nested struct of map[string]interface{} when
						// a property if of the form foo.bar
						if strings.Contains(property, ".") {
							propertyStruct := RetrieveNestedProperty(spec, property)
							properties = propertyStruct
						} else {
							properties[property] = RetrievePropertyDefault(spec, property)
						}
					}
					// Override default spec values with explicit settings from the
					// current bosh deployment manifest, this should be done under each
					// job, inside a `properties` key.
					for propertyName := range properties {
						if explicitSetting, ok := LookUpProperty(job, propertyName); ok {
							properties[propertyName] = explicitSetting
						}
					}
					providerName := provider.Name
					providerType := provider.Type

					// instance_group can override the link name
					if mStruct.InstanceGroups[igID].Jobs[idJob].Provides != nil {
						if value, ok := mStruct.InstanceGroups[igID].Jobs[idJob].Provides[providerName]; ok {
							switch value.(type) {
							case map[interface{}]interface{}:
								if overrideLinkName, ok := value.(map[interface{}]interface{})["as"]; ok {
									providerName = fmt.Sprintf("%v", overrideLinkName)
								}
							default:
								return nil, nil, fmt.Errorf("unexpected type detected: %T, should have been a map", value)
							}

						}
					}

					if providers, ok := providerLinks[providerType]; ok {
						if _, ok := providers[providerName]; ok {
							return nil, nil, fmt.Errorf("multiple providers for link: name=%s type=%s", providerName, providerType)
						}
					}

					if _, ok := providerLinks[providerType]; !ok {
						providerLinks[providerType] = map[string]manifest.JobLink{}
					}

					// convert properties in case they have a type of the form
					// map[interface{}]interface{}, while this will break the
					// JSON marshalling when trying to render the bpm.yml.erb files
					convert(&properties)
					providerLinks[providerType][providerName] = manifest.JobLink{
						Instances:  jobsInstances,
						Properties: properties,
					}
				}
			}
		}
	}
	return releaseSpecs, providerLinks, nil
}

// GetConsumersAndRenderERB will generate a proper context for links and render the required ERB files
func GetConsumersAndRenderERB(mStruct *manifest.Manifest, baseDir string, releaseSpec map[string]map[string]manifest.JobSpec, providerLinks map[string]map[string]manifest.JobLink) error {
	for idIG, ig := range mStruct.InstanceGroups {
		for idJob, job := range ig.Jobs {
			if job.Properties == nil {
				job.Properties = map[string]interface{}{}
			}
			jobPointer := mStruct.InstanceGroups[idIG].Jobs[idJob]

			// Make sure that job.properties.bosh_containerization exists
			if _, ok := jobPointer.Properties["bosh_containerization"]; !ok {
				jobPointer.Properties["bosh_containerization"] = map[string]interface{}{}
			}

			// Make sure that job.properties.bosh_containerization.consumes exists
			if _, ok := jobPointer.Properties["bosh_containerization"].(map[string]interface{})["consumes"]; !ok {
				jobPointer.Properties["bosh_containerization"].(map[string]interface{})["consumes"] = map[string]manifest.JobLink{}
			}

			if lookUpJobRelease(mStruct.Releases, job.Release) {
				jobPointer.Properties["bosh_containerization"].(map[string]interface{})["release"] = job.Release
			}

			spec := releaseSpec[job.Release][job.Name]
			for _, consumes := range spec.Consumes {
				consumesName := consumes.Name
				if jobPointer.Consumes != nil {
					// Deployment manifest can intentionally prevent link resolution as long as the link is optional
					if _, ok := jobPointer.Consumes[consumesName]; !ok {
						if consumes.Optional {
							continue
						}
						return fmt.Errorf("mandatory link of consumer %s is explicitly set to nil", consumesName)
					}

					if _, ok := jobPointer.Consumes[consumesName]; ok {
						if value, ok := job.Consumes[consumesName].(map[interface{}]interface{})["from"]; ok {
							consumesName = value.(string)
						}
					}
				}
				link, hasLink := providerLinks[consumes.Type][consumesName]
				if !hasLink && !consumes.Optional {
					return fmt.Errorf("cannot resolve non-optional link for consumer %s", consumesName)
				}

				jobPointer.Properties["bosh_containerization"].(map[string]interface{})["consumes"] = map[string]manifest.JobLink{
					consumesName: {
						Instances:  link.Instances,
						Properties: link.Properties,
					},
				}
			}

			// ### Render bpm.yml.erb for each job instance
			erbFilePath := filepath.Join(baseDir, "jobs-src", job.Release, job.Name, "templates", "bpm.yml.erb")
			if _, err := os.Stat(erbFilePath); os.IsNotExist(err) {
				return err
			}

			jobSpecfile := filepath.Join(baseDir, "jobs-src", job.Release, job.Name, "job.MF")

			jobInstances := job.Properties["bosh_containerization"].(map[string]interface{})["instances"].([]manifest.JobInstance)
			if jobInstances != nil {
				for i, instance := range jobInstances {
					convert(&jobPointer.Properties)
					renderPointer := btg.NewERBRenderer(
						&btg.EvaluationContext{
							Properties: jobPointer.Properties,
						},

						&btg.InstanceInfo{
							Address:    instance.Address,
							AZ:         instance.AZ,
							ID:         instance.ID,
							Index:      string(instance.Index),
							Deployment: mStruct.Name,
							Name:       instance.Name,
						},

						jobSpecfile,
					)

					// Would be good if we can write the rendered file into memory,
					// rather than to disk

					tmpfile, err := ioutil.TempFile("", "rendered.*.yml")
					if err != nil {
						return err
					}
					defer os.Remove(tmpfile.Name())

					if err := renderPointer.Render(erbFilePath, tmpfile.Name()); err != nil {
						return err
					}

					bpmBytes, err := ioutil.ReadFile(tmpfile.Name())
					if err != nil {
						return err
					}

					jobInstances[i].BPM, err = bpm.NewConfig(bpmBytes)
					if err != nil {
						return err
					}

					// Consider adding a Fingerprint to each job instance
					// instance.Fingerprint = generateSHA(fingerPrintBytes)
				}
			}
			// Store shared bpm as a top level property
			jobInstancesLength := len(jobInstances)
			bpmLastInstance := jobInstances[jobInstancesLength-1].BPM

			for i := range jobInstances {
				if jobInstances[i].BPM == bpmLastInstance {
					jobInstances[i].BPM = nil
				}
			}

			job.Properties["bosh_containerization"].(map[string]interface{})["bpm"] = bpmLastInstance
		}
	}

	// TODO:
	// Deserialize all rendered bpm.yml files
	// Assume the last version may be shared between multiple instance (because it is NOT a bootstrap instance)
	// Delete all instance versions that are identical to the shared one.

	for idIG, ig := range mStruct.InstanceGroups {
		for idJob := range ig.Jobs {
			jobPointer := mStruct.InstanceGroups[idIG].Jobs[idJob]
			convert(&jobPointer.Consumes)
			convert(&jobPointer.Provides)
		}
	}
	manifestResolved, err := json.Marshal(mStruct)
	if err != nil {
		return err
	}

	_ = ioutil.WriteFile("deployment.json", manifestResolved, 0644)

	return nil
}

// generateSHA will generate a new fingerprint based on
// a struct
func generateSHA(fingerPrint []byte) []byte {
	h := md5.New()
	h.Write(fingerPrint)
	bs := h.Sum(nil)
	return bs
}

// convert to be JSON compliant
// When YAML unmarshalling, all properties with nested values will end up
// being of type map[interface{}]interface{}, which is not supported by
// the later JSON.Marshall call in the renderPointer.Render.
// A solution is to enforce a change from map[interface{}]interface{} to
// map[string]interface{}
// Similar problems have been reported, as seen in
// https://github.com/go-yaml/yaml/issues/139
func convert(input *map[string]interface{}) {
	for key, value := range *input {
		switch value.(type) {
		case map[interface{}]interface{}:
			(*input)[key] = cnvrt(value.(map[interface{}]interface{}))
		}
	}
}

// cnvrt is an helper func for convert()
func cnvrt(input map[interface{}]interface{}) map[string]interface{} {
	result := map[string]interface{}{}

	for key, value := range input {
		keyAsString := fmt.Sprintf("%v", key)

		switch value.(type) {
		case map[interface{}]interface{}:
			result[keyAsString] = cnvrt(value.(map[interface{}]interface{}))

		default:
			result[keyAsString] = value
		}
	}

	return result
}

// GatherData will collect different data
// Collect job spec information
// Collect job properties
// Collect bosh links
// Render the bpm yaml file data
func GatherData(mStruct *manifest.Manifest, baseDir string, ns string, desiredIgs []string) error {

	releaseSpecs, providerLinks, err := CollectReleaseSpecsAndProviderLinks(mStruct, baseDir, ns, desiredIgs)
	if err != nil {
		return err
	}
	err = GetConsumersAndRenderERB(mStruct, baseDir, releaseSpecs, providerLinks)
	if err != nil {
		return err
	}
	return nil
}

// LookUpProperty search for property value in the job properties
func LookUpProperty(job manifest.Job, propertyName string) (interface{}, bool) {
	var pointer interface{}

	pointer = job.Properties
	for _, pathPart := range strings.Split(propertyName, ".") {
		switch pointer.(type) {
		case map[string]interface{}:
			hash := pointer.(map[string]interface{})
			if _, ok := hash[pathPart]; !ok {
				return nil, false
			}
			pointer = hash[pathPart]

		case map[interface{}]interface{}:
			hash := pointer.(map[interface{}]interface{})
			if _, ok := hash[pathPart]; !ok {
				return nil, false
			}
			pointer = hash[pathPart]

		default:
			return nil, false
		}
	}
	return pointer, true
}

// RetrieveNestedProperty will generate an nested struct
// based on a string of the type foo.bar
func RetrieveNestedProperty(jobSpec manifest.JobSpec, propertyName string) map[string]interface{} {
	var anStruct map[string]interface{}
	var previous map[string]interface{}
	items := strings.Split(propertyName, ".")
	for i := len(items) - 1; i >= 0; i-- {
		if i == (len(items) - 1) {
			previous = map[string]interface{}{
				items[i]: RetrievePropertyDefault(jobSpec, propertyName),
			}
		} else {
			anStruct = map[string]interface{}{
				items[i]: previous,
			}
			previous = anStruct

		}
	}
	return anStruct
}

// RetrievePropertyDefault return the default value of the spec property
func RetrievePropertyDefault(jobSpec manifest.JobSpec, propertyName string) interface{} {
	if property, ok := jobSpec.Properties[propertyName]; ok {
		return property.Default
	}

	return nil
}

// contains filter instance groups based on the name
func contains(igList []string, name string) bool {
	for _, igName := range igList {
		if name == igName {
			return true
		}
	}
	return false
}

// lookUpJobRelease will check in the main manifest for
// a release name
func lookUpJobRelease(releases []*manifest.Release, jobRelease string) bool {
	for _, release := range releases {
		if release.Name == jobRelease {
			return true
		}
	}

	return false
}
