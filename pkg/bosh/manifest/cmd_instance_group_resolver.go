package manifest

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	btg "github.com/viovanov/bosh-template-go"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
)

// InstanceGroupResolver gathers data for jobs in the manifest, it handles links and returns a deployment manifest
// that only has information pertinent to an instance group.
type InstanceGroupResolver struct {
	baseDir          string
	manifest         Manifest
	instanceGroup    *InstanceGroup
	jobReleaseSpecs  map[string]map[string]JobSpec
	jobProviderLinks JobProviderLinks
}

// NewInstanceGroupResolver returns a data gatherer with logging for a given input manifest and instance group
func NewInstanceGroupResolver(basedir, namespace string, manifest Manifest, instanceGroupName string) (*InstanceGroupResolver, error) {
	ig, err := (&manifest).InstanceGroupByName(instanceGroupName)
	if err != nil {
		return nil, err
	}

	return &InstanceGroupResolver{
		baseDir:          basedir,
		manifest:         manifest,
		instanceGroup:    ig,
		jobReleaseSpecs:  map[string]map[string]JobSpec{},
		jobProviderLinks: JobProviderLinks{},
	}, nil
}

// BPMConfigs returns a map of all BOSH jobs in the instance group
// The output will be persisted by ExtendedJob as 'bpm.yaml' in the
// `<deployment-name>.bpm.<instance-group>-v<version>` secret.
func (dg *InstanceGroupResolver) BPMConfigs() (bpm.Configs, error) {
	bpm := bpm.Configs{}

	err := dg.resolveManifest()
	if err != nil {
		return bpm, err
	}

	for _, job := range dg.instanceGroup.Jobs {
		bpm[job.Name] = *job.Properties.Quarks.BPM
	}

	return bpm, nil
}

// Manifest returns a manifest for a specific instance group only.
// That manifest includes the gathered data from BPM and links.
// The output will be persisted by ExtendedJob as 'properties.yaml' in the
// `<deployment-name>.ig-resolved.<instance-group>-v<version>` secret.
func (dg *InstanceGroupResolver) Manifest() (Manifest, error) {
	err := dg.resolveManifest()
	if err != nil {
		return Manifest{}, err
	}

	return dg.manifest, nil
}

// resolveManifest collects bpm and link information and enriches the manifest accordingly
//
// Data gathered:
// * job spec information
// * job properties
// * bosh links
// * bpm yaml file data
func (dg *InstanceGroupResolver) resolveManifest() error {
	if err := runPreRenderScripts(dg.instanceGroup); err != nil {
		return err
	}

	if err := dg.collectReleaseSpecsAndProviderLinks(); err != nil {
		return err
	}

	if err := dg.processConsumers(); err != nil {
		return err
	}

	if err := dg.renderBPM(); err != nil {
		return err
	}

	return nil
}

// collectReleaseSpecsAndProviderLinks will collect all release specs and generate bosh links for provider jobs
func (dg *InstanceGroupResolver) collectReleaseSpecsAndProviderLinks() error {
	for _, instanceGroup := range dg.manifest.InstanceGroups {
		serviceName := dg.manifest.DNS.HeadlessServiceName(instanceGroup.Name)

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
			// job.properties.quarks
			jobsInstances := instanceGroup.jobInstances(dg.manifest.Name, job.Name, spec)

			// set jobs.properties.quarks.instances with the ig instances
			instanceGroup.Jobs[jobIdx].Properties.Quarks.Instances = jobsInstances

			// Create a list of fully evaluated links provided by the current job
			// These is specified in the job release job.MF file
			if spec.Provides != nil {
				err := dg.jobProviderLinks.Add(job, spec, jobsInstances, serviceName)
				if err != nil {
					return errors.Wrapf(err, "Collecting release spec and provider links failed for %s", job.Name)
				}
			}
		}
	}

	return nil
}

// ProcessConsumers will generate a proper context for links and render the required ERB files
func (dg *InstanceGroupResolver) processConsumers() error {
	for i := range dg.instanceGroup.Jobs {
		job := &dg.instanceGroup.Jobs[i]

		// Verify that the current job release exists on the manifest releases block
		if lookUpJobRelease(dg.manifest.Releases, job.Release) {
			job.Properties.Quarks.Release = job.Release
		}

		err := generateJobConsumersData(job, dg.jobReleaseSpecs, dg.jobProviderLinks)
		if err != nil {
			return errors.Wrapf(err, "Generate Job Consumes data failed for instance group %s", dg.instanceGroup.Name)
		}
	}

	return nil
}

func (dg *InstanceGroupResolver) renderBPM() error {
	for i := range dg.instanceGroup.Jobs {
		job := &dg.instanceGroup.Jobs[i]

		err := dg.renderJobBPM(job, dg.baseDir)
		if err != nil {
			return errors.Wrapf(err, "Rendering BPM failed for instance group %s", dg.instanceGroup.Name)
		}
	}

	return nil
}

// renderJobBPM per job and add its value to the jobInstances.BPM field.
func (dg *InstanceGroupResolver) renderJobBPM(currentJob *Job, baseDir string) error {
	// Location of the current job job.MF file.
	jobSpecFile := filepath.Join(baseDir, "jobs-src", currentJob.Release, currentJob.Name, "job.MF")

	var jobSpec struct {
		Templates map[string]string `yaml:"templates"`
	}

	// First, we must figure out the location of the template.
	// We're looking for a template in the spec, whose result is a file "bpm.yml".
	yamlFile, err := ioutil.ReadFile(jobSpecFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read the job spec file %s in job %s", jobSpecFile, currentJob.Name)
	}
	err = yaml.Unmarshal(yamlFile, &jobSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal the job spec file %s in job %s", jobSpecFile, currentJob.Name)
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
			return errors.Wrapf(err, "os.Stat failed for %s", erbFilePath)
		}

		// Get current job.quarks.instances, which will be required by the renderer to generate
		// the render.InstanceInfo struct.
		jobInstances := currentJob.Properties.Quarks.Instances
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
					Bootstrap:  jobInstance.Bootstrap,
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
				return errors.Wrapf(err, "Creation of tmp file %s failed", tmpfile.Name())
			}
			defer os.Remove(tmpfile.Name())

			if err := renderPointer.Render(erbFilePath, tmpfile.Name()); err != nil {
				return errors.Wrapf(err, "Rendering file %s failed", erbFilePath)
			}

			bpmBytes, err := ioutil.ReadFile(tmpfile.Name())
			if err != nil {
				return errors.Wrapf(err, "Reading of tmp file %s failed", tmpfile.Name())
			}

			// Parse a rendered bpm.yml into the bpm Config struct.
			renderedBPM, err := bpm.NewConfig(bpmBytes)
			if err != nil {
				return errors.Wrapf(err, "Rendering bpm.yaml into bpm config %s failed", string(bpmBytes))
			}

			// Merge processes if they also exist in Quarks
			if currentJob.Properties.Quarks.BPM != nil && len(currentJob.Properties.Quarks.BPM.Processes) > 0 {
				renderedBPM.Processes, err = mergeBPMProcesses(renderedBPM.Processes, currentJob.Properties.Quarks.BPM.Processes)
				if err != nil {
					return errors.Wrapf(err, "failed to merge bpm information from quarks for job '%s'", currentJob.Name)
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
		currentJob.Properties.Quarks.BPM = &firstJobIndexBPM
	} else if currentJob.Properties.Quarks.BPM == nil {
		return errors.Errorf("can't find BPM template for job %s", currentJob.Name)
	}

	return nil
}

// generateJobConsumersData will populate a job with its corresponding provider links
// under properties.quarks.consumes
func generateJobConsumersData(currentJob *Job, jobReleaseSpecs map[string]map[string]JobSpec, jobProviderLinks JobProviderLinks) error {
	currentJobSpecData := jobReleaseSpecs[currentJob.Release][currentJob.Name]
	for _, provider := range currentJobSpecData.Consumes {

		providerName := provider.Name

		if currentJob.Consumes != nil {
			// When the job defines a consumes property in the manifest, use it instead of the one
			// from currentJobSpecData.Consumes.
			if _, ok := currentJob.Consumes[providerName]; ok {
				if value, ok := currentJob.Consumes[providerName].(map[string]interface{})["from"]; ok {
					providerName = value.(string)
				}
			}
		}

		link, hasLink := jobProviderLinks.Lookup(&provider)
		if !hasLink && !provider.Optional {
			return errors.Errorf("cannot resolve non-optional link for provider %s in job %s", providerName, currentJob.Name)
		}

		// generate the job.properties.quarks.consumes struct with the links information from providers.
		if currentJob.Properties.Quarks.Consumes == nil {
			currentJob.Properties.Quarks.Consumes = map[string]JobLink{}
		}

		currentJob.Properties.Quarks.Consumes[providerName] = JobLink{
			Address:    link.Address,
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
				return nil, errors.Wrapf(err, "Failed to merge bpm process information for preset process %s", process.Name)
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
