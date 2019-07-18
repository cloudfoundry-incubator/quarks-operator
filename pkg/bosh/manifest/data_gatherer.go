package manifest

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	btg "github.com/viovanov/bosh-template-go"
	"go.uber.org/zap"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	bc "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/containerization"
)

// JobProviderLinks provides links to other jobs, indexed by provider type and name
type JobProviderLinks map[string]map[string]bc.JobLink

// Lookup returns a link for a type and name, used when links are consumed
func (jpl JobProviderLinks) Lookup(provider *JobSpecProvider) (bc.JobLink, bool) {
	link, ok := jpl[provider.Type][provider.Name]
	return link, ok
}

// Add another job to the lookup map
func (jpl JobProviderLinks) Add(job Job, spec JobSpec, jobsInstances []bc.JobInstance) error {
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
				case map[interface{}]interface{}:
					if overrideLinkName, ok := value["as"]; ok {
						linkName = fmt.Sprintf("%v", overrideLinkName)
					}
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
			jpl[linkType] = map[string]bc.JobLink{}
		}

		// construct the jobProviderLinks of the current job that provides
		// a link
		jpl[linkType][linkName] = bc.JobLink{
			Instances:  jobsInstances,
			Properties: properties,
		}
	}
	return nil
}

// DataGatherer gathers data for jobs in the manifest, it handles links and returns a deployment manifest
// that only has information pertinent to an instance group.
type DataGatherer struct {
	log           *zap.SugaredLogger
	baseDir       string
	manifest      Manifest
	namespace     string
	instanceGroup *InstanceGroup

	jobReleaseSpecs  map[string]map[string]JobSpec
	jobProviderLinks JobProviderLinks
}

// NewDataGatherer returns a data gatherer with logging for a given input manifest and instance group
func NewDataGatherer(log *zap.SugaredLogger, basedir, namespace string, manifest Manifest, instanceGroupName string) (*DataGatherer, error) {
	ig, err := (&manifest).InstanceGroupByName(instanceGroupName)
	if err != nil {
		return nil, err
	}

	return &DataGatherer{
		log:              log,
		baseDir:          basedir,
		manifest:         manifest,
		namespace:        namespace,
		instanceGroup:    ig,
		jobReleaseSpecs:  map[string]map[string]JobSpec{},
		jobProviderLinks: JobProviderLinks{},
	}, nil
}

// BPMConfigs returns a map of all BOSH jobs in the instance group
// The output will be persisted by ExtendedJob as 'bpm.yaml' in the
// `<deployment-name>.bpm.<instance-group>-v<version>` secret.
func (dg *DataGatherer) BPMConfigs() (bpm.Configs, error) {
	bpm := bpm.Configs{}

	err := dg.gatherData()
	if err != nil {
		return bpm, err
	}

	for _, job := range dg.instanceGroup.Jobs {
		bpm[job.Name] = *job.Properties.BOSHContainerization.BPM
	}

	return bpm, nil
}

// ResolvedProperties returns a manifest for a specific instance group only.
// That manifest includes the gathered data from BPM and links.
// The output will be persisted by ExtendedJob as 'properties.yaml' in the
// `<deployment-name>.ig-resolved.<instance-group>-v<version>` secret.
func (dg *DataGatherer) ResolvedProperties() (Manifest, error) {
	err := dg.gatherData()
	if err != nil {
		return Manifest{}, err
	}

	return dg.manifest, nil
}

// gatherData collects bpm and link information and enriches the manifest accordingly
//
// Data gathered:
// * job spec information
// * job properties
// * bosh links
// * bpm yaml file data
func (dg *DataGatherer) gatherData() error {
	err := dg.collectReleaseSpecsAndProviderLinks()
	if err != nil {
		return err
	}

	err = dg.processConsumers()
	if err != nil {
		return err
	}

	err = dg.renderBPM()
	if err != nil {
		return err
	}

	return nil
}

// collectReleaseSpecsAndProviderLinks will collect all release specs and generate bosh links for provider jobs
func (dg *DataGatherer) collectReleaseSpecsAndProviderLinks() error {
	for _, instanceGroup := range dg.manifest.InstanceGroups {
		for jobIdx, job := range instanceGroup.Jobs {
			// make sure a map entry exists for the current job release
			if _, ok := dg.jobReleaseSpecs[job.Release]; !ok {
				dg.jobReleaseSpecs[job.Release] = map[string]JobSpec{}
			}

			// load job.MF into jobReleaseSpecs[job.Release][job.Name]
			if _, ok := dg.jobReleaseSpecs[job.Release][job.Name]; !ok {
				jobSpec, err := job.loadSpec(dg.baseDir)
				if err != nil {
					return err
				}
				dg.jobReleaseSpecs[job.Release][job.Name] = *jobSpec
			}

			// spec of the current jobs release/name
			spec := dg.jobReleaseSpecs[job.Release][job.Name]

			// Generate instance spec for each ig instance
			// This will be stored inside the current job under
			// job.properties.bosh_containerization
			jobsInstances := instanceGroup.jobInstances(dg.namespace, dg.manifest.Name, job.Name, spec)

			// set jobs.properties.bosh_containerization.instances with the ig instances
			instanceGroup.Jobs[jobIdx].Properties.BOSHContainerization.Instances = jobsInstances

			// Create a list of fully evaluated links provided by the current job
			// These is specified in the job release job.MF file
			if spec.Provides != nil {
				err := dg.jobProviderLinks.Add(job, spec, jobsInstances)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ProcessConsumers will generate a proper context for links and render the required ERB files
func (dg *DataGatherer) processConsumers() error {
	for i := range dg.instanceGroup.Jobs {
		job := &dg.instanceGroup.Jobs[i]

		// Verify that the current job release exists on the manifest releases block
		if lookUpJobRelease(dg.manifest.Releases, job.Release) {
			job.Properties.BOSHContainerization.Release = job.Release
		}

		err := generateJobConsumersData(job, dg.jobReleaseSpecs, dg.jobProviderLinks)
		if err != nil {
			return err
		}
	}

	return nil
}

func (dg *DataGatherer) renderBPM() error {
	for i := range dg.instanceGroup.Jobs {
		job := &dg.instanceGroup.Jobs[i]

		err := dg.renderJobBPM(job, dg.baseDir)
		if err != nil {
			return err
		}
	}

	return nil
}

// renderJobBPM per job and add its value to the jobInstances.BPM field.
func (dg *DataGatherer) renderJobBPM(currentJob *Job, baseDir string) error {
	// Run pre-render scripts for the current job.
	for idx, script := range currentJob.Properties.BOSHContainerization.PreRenderScripts {
		if err := runPreRenderScript(script, idx, true); err != nil {
			return errors.Wrapf(err, "failed to run pre-render script %d for job %s", idx, currentJob.Name)
		}
	}

	// Location of the current job job.MF file.
	jobSpecFile := filepath.Join(baseDir, "jobs-src", currentJob.Release, currentJob.Name, "job.MF")

	var jobSpec struct {
		Templates map[string]string `yaml:"templates"`
	}

	// First, we must figure out the location of the template.
	// We're looking for a template in the spec, whose result is a file "bpm.yml".
	yamlFile, err := ioutil.ReadFile(jobSpecFile)
	if err != nil {
		return errors.Wrap(err, "failed to read the job spec file")
	}
	err = yaml.Unmarshal(yamlFile, &jobSpec)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal the job spec file")
	}

	var bpmSource string
	for srcFile, dstFile := range jobSpec.Templates {
		if filepath.Base(dstFile) == "bpm.yml" {
			bpmSource = srcFile
			break
		}
	}

	if bpmSource != "" {
		// Render bpm.yml.erb for each job instance.
		erbFilePath := filepath.Join(baseDir, "jobs-src", currentJob.Release, currentJob.Name, "templates", bpmSource)
		if _, err := os.Stat(erbFilePath); err != nil {
			return err
		}

		// Get current job.bosh_containerization.instances, which will be required by the renderer to generate
		// the render.InstanceInfo struct.
		jobInstances := currentJob.Properties.BOSHContainerization.Instances
		if jobInstances == nil {
			return nil
		}

		jobIndexBPM := make([]bpm.Config, len(jobInstances))
		for i, jobInstance := range jobInstances {

			properties := currentJob.Properties.ToMap()

			renderPointer := btg.NewERBRenderer(
				&btg.EvaluationContext{
					Properties: properties,
				},

				&btg.InstanceInfo{
					Address:    jobInstance.Address,
					AZ:         jobInstance.AZ,
					Bootstrap:  jobInstance.Index == 0,
					ID:         jobInstance.ID,
					Index:      jobInstance.Index,
					Deployment: dg.manifest.Name,
					Name:       jobInstance.Name,
				},

				jobSpecFile,
			)

			// Write to a tmp, this is following the conventions on how the
			// https://github.com/viovanov/bosh-template-go/ processes the params
			// when we calling the *.Render().
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

			// Parse a rendered bpm.yml into the bpm Config struct.
			renderedBPM, err := bpm.NewConfig(bpmBytes)
			if err != nil {
				return err
			}

			// Merge processes if they also exist in BOSHContainerization
			if currentJob.Properties.BOSHContainerization.BPM != nil && len(currentJob.Properties.BOSHContainerization.BPM.Processes) > 0 {
				renderedBPM.Processes, err = mergeBPMProcesses(renderedBPM.Processes, currentJob.Properties.BOSHContainerization.BPM.Processes)
				if err != nil {
					return errors.Wrapf(err, "failed to merge bpm information from bosh_containerization for job '%s'", currentJob.Name)
				}
			}

			jobIndexBPM[i] = renderedBPM
		}

		firstJobIndexBPM := jobIndexBPM[0]
		for _, jobBPMInstance := range jobIndexBPM {
			if !reflect.DeepEqual(jobBPMInstance, firstJobIndexBPM) {
				firstJobIndexBPM.UnsupportedTemplate = true
			}
		}
		currentJob.Properties.BOSHContainerization.BPM = &firstJobIndexBPM
	} else if currentJob.Properties.BOSHContainerization.BPM == nil {
		return fmt.Errorf("can't find BPM template for job %s", currentJob.Name)
	}

	return nil
}

// generateJobConsumersData will populate a job with its corresponding provider links
// under properties.bosh_containerization.consumes
func generateJobConsumersData(currentJob *Job, jobReleaseSpecs map[string]map[string]JobSpec, jobProviderLinks JobProviderLinks) error {
	currentJobSpecData := jobReleaseSpecs[currentJob.Release][currentJob.Name]
	for _, provider := range currentJobSpecData.Consumes {

		providerName := provider.Name

		if currentJob.Consumes != nil {
			// Deployment manifest can intentionally prevent link resolution as long as the link is optional
			// Continue to the next job if this one does not consumes links.
			if _, ok := currentJob.Consumes[providerName]; !ok {
				if provider.Optional {
					continue
				}
				return fmt.Errorf("mandatory link of consumer %s is explicitly set to nil", providerName)
			}

			// When the job defines a consumes property in the manifest, use it instead of the one
			// from currentJobSpecData.Consumes.
			if _, ok := currentJob.Consumes[providerName]; ok {
				if value, ok := currentJob.Consumes[providerName].(map[interface{}]interface{})["from"]; ok {
					providerName = value.(string)
				}
			}
		}

		link, hasLink := jobProviderLinks.Lookup(&provider)
		if !hasLink && !provider.Optional {
			return fmt.Errorf("cannot resolve non-optional link for provider %s", providerName)
		}

		// generate the job.properties.bosh_containerization.consumes struct with the links information from providers.
		if currentJob.Properties.BOSHContainerization.Consumes == nil {
			currentJob.Properties.BOSHContainerization.Consumes = map[string]bc.JobLink{}
		}

		currentJob.Properties.BOSHContainerization.Consumes[providerName] = bc.JobLink{
			Instances:  link.Instances,
			Properties: link.Properties,
		}
	}
	return nil
}

// Property search for property value in the job properties
func (job Job) Property(propertyName string) (interface{}, bool) {
	var pointer interface{}

	pointer = job.Properties.Properties
	for _, pathPart := range strings.Split(propertyName, ".") {
		switch pointerCast := pointer.(type) {
		case map[string]interface{}:
			if _, ok := pointerCast[pathPart]; !ok {
				return nil, false
			}
			pointer = pointerCast[pathPart]

		case map[interface{}]interface{}:
			if _, ok := pointerCast[pathPart]; !ok {
				return nil, false
			}
			pointer = pointerCast[pathPart]

		default:
			return nil, false
		}
	}
	return pointer, true
}

// RetrieveNestedProperty will generate a nested struct
// based on a string of the type foo.bar in the provided map
// It overrides existing property paths that are not of the correct type.
func (js JobSpec) RetrieveNestedProperty(properties map[string]interface{}, propertyName string) {
	items := strings.Split(propertyName, ".")
	currentLevel := properties

	for idx, gram := range items {
		if idx == len(items)-1 {
			currentLevel[gram] = js.RetrievePropertyDefault(propertyName)
			return
		}

		// Path doesn't exist, create it
		if _, ok := currentLevel[gram]; !ok {
			currentLevel[gram] = map[string]interface{}{}
		}

		// This is not the leaf, and we must make sure we have a map
		if _, ok := currentLevel[gram].(map[string]interface{}); !ok {
			currentLevel[gram] = map[string]interface{}{}
		}

		currentLevel = currentLevel[gram].(map[string]interface{})
	}
}

// RetrievePropertyDefault return the default value of the spec property
func (js JobSpec) RetrievePropertyDefault(propertyName string) interface{} {
	if property, ok := js.Properties[propertyName]; ok {
		return property.Default
	}

	return nil
}

// lookUpJobRelease will check in the main manifest for
// a release name
func lookUpJobRelease(releases []*Release, jobRelease string) bool {
	for _, release := range releases {
		if release.Name == jobRelease {
			return true
		}
	}

	return false
}

// mergeNestedExplicitProperty merges an explicitly set Job property into an existing
// map of properties
func mergeNestedExplicitProperty(properties map[string]interface{}, job Job, propertyName string) {
	items := strings.Split(propertyName, ".")
	currentLevel := properties

	for idx, gram := range items {
		if idx == len(items)-1 {
			if value, ok := job.Property(propertyName); ok {
				currentLevel[gram] = value
			}
			return
		}

		// Path doesn't exist, create it
		if _, ok := currentLevel[gram]; !ok {
			currentLevel[gram] = map[string]interface{}{}
		}

		// This is not the leaf, and we must make sure we have a map
		if _, ok := currentLevel[gram].(map[string]interface{}); !ok {
			currentLevel[gram] = map[string]interface{}{}
		}

		currentLevel = currentLevel[gram].(map[string]interface{})
	}
}

// mergeBPMProcesses will return new processes slice which be overwritten with preset processes
func mergeBPMProcesses(renderedProcesses []bpm.Process, presetProcesses []bpm.Process) ([]bpm.Process, error) {
	for _, process := range presetProcesses {
		index, exist := indexOfBPMProcess(renderedProcesses, process.Name)
		if exist {
			err := mergo.MergeWithOverwrite(&renderedProcesses[index], process)
			if err != nil {
				return nil, errors.Wrap(err, "failed to merge bpm process information")
			}
		} else {
			renderedProcesses = append(renderedProcesses, process)
		}
	}

	return renderedProcesses, nil
}

// indexOfBPMProcess will return the first index at which a given process name can be found in the []bpm.Process.
// Return -1 if not find valid version
func indexOfBPMProcess(processes []bpm.Process, processName string) (int, bool) {
	for i, process := range processes {
		if process.Name == processName {
			return i, true
		}
	}

	return -1, false
}
