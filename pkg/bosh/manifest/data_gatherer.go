package manifest

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	btg "github.com/viovanov/bosh-template-go"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// JobProviderLinks provides links to other jobs, indexed by provider type and name
type JobProviderLinks map[string]map[string]JobLink

// Lookup returns a link for a type and name, used when links are consumed
func (jpl JobProviderLinks) Lookup(consumesType string, consumesName string) (JobLink, bool) {
	link, ok := jpl[consumesType][consumesName]
	return link, ok
}

// Add another job to the lookup map
func (jpl JobProviderLinks) Add(job Job, spec JobSpec, jobsInstances []JobInstance) error {
	var properties map[string]interface{}

	for _, provider := range spec.Provides {
		properties = map[string]interface{}{}
		for _, property := range provider.Properties {
			// generate a nested struct of map[string]interface{} when
			// a property is of the form foo.bar
			if strings.Contains(property, ".") {
				propertyStruct := spec.RetrieveNestedProperty(property)
				properties = propertyStruct
			} else {
				properties[property] = spec.RetrievePropertyDefault(property)
			}
		}
		// Override default spec values with explicit settings from the
		// current bosh deployment manifest, this should be done under each
		// job, inside a `properties` key.
		for propertyName := range properties {
			if explicitSetting, ok := job.Property(propertyName); ok {
				properties[propertyName] = explicitSetting
			}
		}
		providerName := provider.Name
		providerType := provider.Type

		// instance_group.job can override the link name through the
		// instance_group.job.provides, via the "as" key
		if job.Provides != nil {
			if value, ok := job.Provides[providerName]; ok {
				switch value := value.(type) {
				case map[interface{}]interface{}:
					if overrideLinkName, ok := value["as"]; ok {
						providerName = fmt.Sprintf("%v", overrideLinkName)
					}
				default:
					return fmt.Errorf("unexpected type detected: %T, should have been a map", value)
				}

			}
		}

		if providers, ok := jpl[providerType]; ok {
			if _, ok := providers[providerName]; ok {
				return fmt.Errorf("multiple providers for link: name=%s type=%s", providerName, providerType)
			}
		}

		if _, ok := jpl[providerType]; !ok {
			jpl[providerType] = map[string]JobLink{}
		}

		// construct the jobProviderLinks of the current job that provides
		// a link
		jpl[providerType][providerName] = JobLink{
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
func (dg *DataGatherer) BPMConfigs() (bpm.Configs, error) {
	bpm := bpm.Configs{}

	err := dg.gatherData()
	if err != nil {
		return bpm, err
	}

	for _, job := range dg.instanceGroup.Jobs {
		bpm[job.Name] = job.Properties.BOSHContainerization.BPM
	}

	return bpm, nil
}

// ResolvedProperties returns the manifest including the gathered data
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

// CollectReleaseSpecsAndProviderLinks will collect all release specs and generate bosh links for provider jobs
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

// renderJobBPM per job and add its value to the jobInstances.BPM field
func (dg *DataGatherer) renderJobBPM(currentJob *Job, baseDir string) error {
	// Location of the current job job.MF file
	jobSpecFile := filepath.Join(baseDir, "jobs-src", currentJob.Release, currentJob.Name, "job.MF")

	var jobSpec struct {
		Templates map[string]string `yaml:"templates"`
	}

	// First, we must figure out the location of the template.
	// We're looking for a template in the spec, whose result is a file "bpm.yml"
	yamlFile, err := ioutil.ReadFile(jobSpecFile)
	if err != nil {
		return errors.Wrap(err, "failed to read the job spec file")
	}
	err = yaml.Unmarshal(yamlFile, &jobSpec)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal the job spec file")
	}

	bpmSource := ""
	for srcFile, dstFile := range jobSpec.Templates {
		if filepath.Base(dstFile) == "bpm.yml" {
			bpmSource = srcFile
			break
		}
	}

	if bpmSource == "" {
		return fmt.Errorf("can't find BPM template for job %s", currentJob.Name)
	}

	// Render bpm.yml.erb for each job instance
	erbFilePath := filepath.Join(baseDir, "jobs-src", currentJob.Release, currentJob.Name, "templates", bpmSource)
	if _, err := os.Stat(erbFilePath); err != nil {
		return err
	}

	// Get current job.bosh_containerization.instances, which will be required by the renderer to generate
	// the render.InstanceInfo struct
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
				ID:         jobInstance.ID,
				Index:      string(jobInstance.Index),
				Deployment: dg.manifest.Name,
				Name:       jobInstance.Name,
			},

			jobSpecFile,
		)

		// Write to a tmp, this is following the conventions on how the
		// https://github.com/viovanov/bosh-template-go/ processes the params
		// when we calling the *.Render()
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

		// Parse a rendered bpm.yml into the bpm Config struct
		renderedBPM, err := bpm.NewConfig(bpmBytes)
		if err != nil {
			return err
		}

		// Overwrite Processes if BPM.Processes exists in BOSHContainerization
		if len(currentJob.Properties.BOSHContainerization.BPM.Processes) > 0 {
			renderedBPM.Processes = overwriteBPMProcesses(renderedBPM.Processes, currentJob.Properties.BOSHContainerization.BPM.Processes)
		}

		jobIndexBPM[i] = renderedBPM
	}

	for _, jobBPMInstance := range jobIndexBPM {
		if !reflect.DeepEqual(jobBPMInstance, jobIndexBPM[0]) {
			dg.log.Warnf("found different BPM job indexes for job %s in manifest %s, this is NOT SUPPORTED", currentJob.Name, dg.manifest.Name)
		}
	}
	currentJob.Properties.BOSHContainerization.BPM = jobIndexBPM[0]

	return nil
}

// generateJobConsumersData will populate a job with its corresponding provider links
// under properties.bosh_containerization.consumes
func generateJobConsumersData(currentJob *Job, jobReleaseSpecs map[string]map[string]JobSpec, jobProviderLinks JobProviderLinks) error {
	currentJobSpecData := jobReleaseSpecs[currentJob.Release][currentJob.Name]
	for _, consumes := range currentJobSpecData.Consumes {

		consumesName := consumes.Name

		if currentJob.Consumes != nil {
			// Deployment manifest can intentionally prevent link resolution as long as the link is optional
			// Continue to the next job if this one does not consumes links
			if _, ok := currentJob.Consumes[consumesName]; !ok {
				if consumes.Optional {
					continue
				}
				return fmt.Errorf("mandatory link of consumer %s is explicitly set to nil", consumesName)
			}

			// When the job defines a consumes property in the manifest, use it instead of the one
			// from currentJobSpecData.Consumes
			if _, ok := currentJob.Consumes[consumesName]; ok {
				if value, ok := currentJob.Consumes[consumesName].(map[interface{}]interface{})["from"]; ok {
					consumesName = value.(string)
				}
			}
		}

		link, hasLink := jobProviderLinks.Lookup(consumes.Type, consumesName)
		if !hasLink && !consumes.Optional {
			return fmt.Errorf("cannot resolve non-optional link for consumer %s", consumesName)
		}

		// generate the job.properties.bosh_containerization.consumes struct with the links information from providers.
		if currentJob.Properties.BOSHContainerization.Consumes == nil {
			currentJob.Properties.BOSHContainerization.Consumes = map[string]JobLink{}
		}

		currentJob.Properties.BOSHContainerization.Consumes[consumesName] = JobLink{
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

// RetrieveNestedProperty will generate an nested struct
// based on a string of the type foo.bar
func (js JobSpec) RetrieveNestedProperty(propertyName string) map[string]interface{} {
	var anStruct map[string]interface{}
	var previous map[string]interface{}
	items := strings.Split(propertyName, ".")
	for i := len(items) - 1; i >= 0; i-- {
		if i == (len(items) - 1) {
			previous = map[string]interface{}{
				items[i]: js.RetrievePropertyDefault(propertyName),
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

// overwriteBPMProcesses will return new processes slice which be overwritten with preset processes
func overwriteBPMProcesses(renderedProcesses []bpm.Process, presetProcesses []bpm.Process) []bpm.Process {
	for _, process := range presetProcesses {
		index, exist := indexOfBPMProcess(renderedProcesses, process.Name)
		if exist {
			renderedProcesses[index] = process
		} else {
			renderedProcesses = append(renderedProcesses, process)
		}
	}

	return renderedProcesses
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
