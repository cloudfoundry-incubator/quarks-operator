package manifest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
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
	jobProviderLinks jobProviderLinks
	fs               afero.Fs
}

// NewInstanceGroupResolver returns a data gatherer with logging for a given input manifest and instance group
func NewInstanceGroupResolver(fs afero.Fs, basedir string, manifest Manifest, instanceGroupName string) (*InstanceGroupResolver, error) {
	ig, found := manifest.InstanceGroups.InstanceGroupByName(instanceGroupName)
	if !found {
		return nil, errors.Errorf("instance group '%s' not found", instanceGroupName)
	}

	return &InstanceGroupResolver{
		baseDir:          basedir,
		manifest:         manifest,
		instanceGroup:    ig,
		jobReleaseSpecs:  map[string]map[string]JobSpec{},
		jobProviderLinks: newJobProviderLinks(),
		fs:               fs,
	}, nil
}

// BPMInfo returns an instance of BPMInfo which consists info about instances,
// azs, env, variables and a map of all BOSH jobs in the instance group.
// The output will be persisted by QuarksJob as 'bpm.yaml' in the
// `<deployment-name>.bpm.<instance-group>-v<version>` secret.
func (igr *InstanceGroupResolver) BPMInfo(initialRollout bool) (BPMInfo, error) {
	bpmInfo := BPMInfo{}

	err := igr.resolveManifest(initialRollout)
	if err != nil {
		return bpmInfo, err
	}

	bpmInfo.Configs = bpm.Configs{}
	for _, job := range igr.instanceGroup.Jobs {
		if job.Properties.Quarks.BPM == nil {
			return bpmInfo, errors.Errorf("Empty bpm configs about job '%s'", job.Name)
		}
		bpmInfo.Configs[job.Name] = *job.Properties.Quarks.BPM
	}

	bpmInfo.InstanceGroup.Name = igr.instanceGroup.Name
	bpmInfo.InstanceGroup.AZs = igr.instanceGroup.AZs
	bpmInfo.InstanceGroup.Instances = igr.instanceGroup.Instances
	bpmInfo.InstanceGroup.Env = igr.instanceGroup.Env
	bpmInfo.Variables = igr.manifest.Variables

	return bpmInfo, nil
}

// Manifest returns a manifest for a specific instance group only.
// That manifest includes the gathered data from BPM and links.
// The output will be persisted by QuarksJob as 'properties.yaml' in the
// `<deployment-name>.ig-resolved.<instance-group>-v<version>` secret.
func (igr *InstanceGroupResolver) Manifest(initialRollout bool) (Manifest, error) {
	err := igr.resolveManifest(initialRollout)
	if err != nil {
		return Manifest{}, err
	}

	// Filter igManifest to contain only relevant fields
	igJobs := []Job{}
	for _, job := range igr.instanceGroup.Jobs {

		igQuarks := Quarks{
			Consumes:         job.Properties.Quarks.Consumes,
			PreRenderScripts: job.Properties.Quarks.PreRenderScripts,
		}

		igJobProperties := JobProperties{
			Properties: job.Properties.Properties,
			Quarks:     igQuarks,
		}

		igJob := Job{
			Name:       job.Name,
			Release:    job.Release,
			Properties: igJobProperties,
		}

		igJobs = append(igJobs, igJob)
	}

	ig := &InstanceGroup{Name: igr.instanceGroup.Name, Jobs: igJobs}

	igManifest := Manifest{
		Name:           igr.manifest.Name,
		InstanceGroups: []*InstanceGroup{ig},
	}

	return igManifest, nil
}

// SaveLinks writes provides.json with all links for this instance group
func (igr *InstanceGroupResolver) SaveLinks(path string) error {
	//path := "/mnt/quarks/provides.json"
	path = filepath.Join(path, "provides.json")
	igName := igr.instanceGroup.Name

	properties := igr.jobProviderLinks.instanceGroups[igName]

	var result = map[string]string{}
	for id, property := range properties {
		jsonBytes, err := json.Marshal(property)
		if err != nil {
			return errors.Wrapf(err, "JSON marshalling failed for ig '%s' property '%s'", igName, id)
		}

		result[id] = string(jsonBytes)
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return errors.Wrapf(err, "JSON marshalling failed for ig '%s' properties", igName)
	}

	err = afero.WriteFile(igr.fs, path, jsonBytes, 0644)
	if err != nil {
		return errors.Wrapf(err, "Failed to write JSON to a output file '%s'", path)
	}

	return nil
}

// resolveManifest collects bpm and link information and enriches the manifest accordingly
//
// Data gathered:
// * job spec information
// * job properties
// * bosh links
// * bpm yaml file data
func (igr *InstanceGroupResolver) resolveManifest(initialRollout bool) error {
	if err := runPreRenderScripts(igr.instanceGroup); err != nil {
		return err
	}

	if err := igr.collectReleaseSpecsAndProviderLinks(initialRollout); err != nil {
		return err
	}

	if err := igr.processConsumers(); err != nil {
		return err
	}

	if err := igr.renderBPM(); err != nil {
		return err
	}

	return nil
}

// CollectQuarksLinks collects all links from a directory specified by path
func (igr *InstanceGroupResolver) CollectQuarksLinks(linksPath string) error {
	exist, err := afero.DirExists(igr.fs, linksPath)
	if err != nil {
		return errors.Wrapf(err, "could not check if path '%s' exists", linksPath)
	}
	if !exist {
		return nil
	}

	links, err := afero.ReadDir(igr.fs, linksPath)
	if err != nil {
		return errors.Wrapf(err, "could not read links directory '%s'", linksPath)
	}

	quarksLinks, ok := igr.manifest.Properties["quarks_links"]
	if !ok {
		return fmt.Errorf("missing quarks_links key in manifest properties")
	}
	qs, ok := quarksLinks.(map[string]interface{})
	if !ok {
		return fmt.Errorf("could not get a map of QuarksLink")
	}

	// Assume we have a list of files named as the provider names of a link
	for _, l := range links {
		if l.IsDir() {
			linkName := l.Name()
			qMap, ok := qs[linkName].(map[string]interface{})
			if !ok {
				return fmt.Errorf("could not cast for link %s", linkName)
			}

			q, err := getQuarksLinkFromMap(qMap)
			if err != nil {
				return fmt.Errorf("could not get quarks link '%s' from map", linkName)
			}

			linkType := q.Type
			properties := map[string]interface{}{}
			properties[linkName] = map[string]interface{}{}
			linkP := map[string]interface{}{}
			err = afero.Walk(igr.fs, filepath.Clean(filepath.Join(linksPath, l.Name())), func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !info.IsDir() {
					_, propertyFileName := filepath.Split(path)
					// Skip the symlink to a directory
					if strings.HasPrefix(propertyFileName, "..") {
						return nil
					}
					varBytes, err := afero.ReadFile(igr.fs, path)
					if err != nil {
						return errors.Wrapf(err, "could not read link %s", l.Name())
					}

					linkP[propertyFileName] = string(varBytes)
				}
				return nil
			})
			if err != nil {
				return errors.Wrapf(err, "walking links path")
			}

			properties[linkName] = linkP
			igr.jobProviderLinks.AddExternalLink(linkName, linkType, q.Address, q.Instances, properties)
		}
	}

	return nil
}

// collectReleaseSpecsAndProviderLinks will collect all release specs and generate bosh links for provider jobs
func (igr *InstanceGroupResolver) collectReleaseSpecsAndProviderLinks(initialRollout bool) error {
	for _, instanceGroup := range igr.manifest.InstanceGroups {
		serviceName := igr.manifest.DNS.HeadlessServiceName(instanceGroup.Name)

		for jobIdx, job := range instanceGroup.Jobs {
			// make sure a map entry exists for the current job release
			if _, ok := igr.jobReleaseSpecs[job.Release]; !ok {
				igr.jobReleaseSpecs[job.Release] = map[string]JobSpec{}
			}

			// load job.MF into jobReleaseSpecs[job.Release][job.Name]
			if _, ok := igr.jobReleaseSpecs[job.Release][job.Name]; !ok {
				jobSpec, err := job.loadSpec(igr.baseDir)
				if err != nil {
					return err
				}
				igr.jobReleaseSpecs[job.Release][job.Name] = *jobSpec
			}

			// spec of the current jobs release/name
			spec := igr.jobReleaseSpecs[job.Release][job.Name]

			// Generate instance spec for each ig instance
			// This will be stored inside the current job under
			// job.properties.quarks
			jobsInstances := instanceGroup.jobInstances(igr.manifest.Name, job.Name, initialRollout)

			// set jobs.properties.quarks.instances with the ig instances
			instanceGroup.Jobs[jobIdx].Properties.Quarks.Instances = jobsInstances

			// Create a list of fully evaluated links provided by the current job
			// These is specified in the job release job.MF file
			if spec.Provides != nil {
				err := igr.jobProviderLinks.Add(instanceGroup.Name, job, spec, jobsInstances, serviceName)
				if err != nil {
					return errors.Wrapf(err, "Collecting release spec and provider links failed for %s", job.Name)
				}
			}
		}
	}
	return nil
}

// ProcessConsumers will generate a proper context for links and render the required ERB files
func (igr *InstanceGroupResolver) processConsumers() error {
	for i := range igr.instanceGroup.Jobs {
		job := &igr.instanceGroup.Jobs[i]

		// Verify that the current job release exists on the manifest releases block
		if lookUpJobRelease(igr.manifest.Releases, job.Release) {
			job.Properties.Quarks.Release = job.Release
		}

		err := generateJobConsumersData(job, igr.jobReleaseSpecs, igr.jobProviderLinks)
		if err != nil {
			return errors.Wrapf(err, "Generate Job Consumes data failed for instance group %s", igr.instanceGroup.Name)
		}
	}

	return nil
}

func (igr *InstanceGroupResolver) renderBPM() error {
	for i := range igr.instanceGroup.Jobs {
		job := &igr.instanceGroup.Jobs[i]

		err := igr.renderJobBPM(job, igr.baseDir)
		if err != nil {
			return errors.Wrapf(err, "Rendering BPM failed for instance group %s", igr.instanceGroup.Name)
		}
	}

	return nil
}

// renderJobBPM per job and add its value to the jobInstances.BPM field.
func (igr *InstanceGroupResolver) renderJobBPM(currentJob *Job, baseDir string) error {
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
					ID:         jobInstance.ID,
					Index:      jobInstance.Index,
					Deployment: igr.manifest.Name,
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
		err = validateBPMProcesses(firstJobIndexBPM.Processes)
		if err != nil {
			return errors.Wrapf(err, "invalid BPM process for job %s", currentJob.Name)
		}
		currentJob.Properties.Quarks.BPM = &firstJobIndexBPM
	} else if currentJob.Properties.Quarks.BPM == nil {
		return errors.Errorf("can't find BPM template for job %s", currentJob.Name)
	}

	return nil
}

func validateBPMProcesses(processes []bpm.Process) error {
	for _, process := range processes {
		if process.Executable == "" {
			return errors.Errorf("no executable specified for process %s", process.Name)
		}
	}
	return nil
}

// generateJobConsumersData will populate a job with its corresponding provider links
// under properties.quarks.consumes
func generateJobConsumersData(currentJob *Job, jobReleaseSpecs map[string]map[string]JobSpec, jobProviderLinks jobProviderLinks) error {
	currentJobSpecData := jobReleaseSpecs[currentJob.Release][currentJob.Name]
	for _, provider := range currentJobSpecData.Consumes {
		providerName := getProviderNameFromConsumer(*currentJob, provider.Name)

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

// getProviderNameFromConsumer get the override of the provider to consume.
// This should match the name defined in the provider's release spec or the name defined by provider's as property
func getProviderNameFromConsumer(job Job, provider string) string {
	// When the job defines a consumes property in the manifest, use it instead of the provider
	// from currentJobSpecData.Consumes.
	if job.Consumes == nil {
		return provider
	}

	consumes, ok := job.Consumes[provider]
	if !ok {
		return provider
	}

	c, ok := consumes.(map[string]interface{})
	if !ok {
		return provider
	}

	override, ok := c["from"]
	if !ok {
		return provider
	}

	providerName, _ := override.(string)
	if len(providerName) == 0 {
		return provider
	}

	return providerName
}

func getQuarksLinkFromMap(m map[string]interface{}) (QuarksLink, error) {
	var result QuarksLink

	data, err := json.Marshal(m)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}
