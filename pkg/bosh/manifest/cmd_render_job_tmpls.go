package manifest

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	btg "github.com/viovanov/bosh-template-go"
	"golang.org/x/sync/errgroup"
)

const (
	typeBPM        = "bpm"
	typeIGResolver = "ig_resolver"
	typeJobs       = "jobs"
)

// RenderJobTemplates will render templates for all jobs of the instance group
// https://bosh.io/docs/create-release/#job-specs
// boshManifest is a resolved manifest for a single instance group
func RenderJobTemplates(
	boshManifestPath string,
	jobsDir string,
	jobsOutputDir string,
	instanceGroupName string,
	specIndex int,
	podIP net.IP,
	replicas int,
	initialRollout bool,
) error {
	if podIP == nil {
		return fmt.Errorf("the pod IP is empty")
	}

	if err := btg.CheckRubyAvailable(); err != nil {
		return errors.Wrap(err, "ruby is not available")
	}

	if err := btg.CheckBOSHTemplateGemAvailable(); err != nil {
		return errors.Wrap(err, "bosh template gem is not available")
	}

	// Loading deployment manifest file
	resolvedYML, err := ioutil.ReadFile(boshManifestPath)
	if err != nil {
		return errors.Wrapf(err, "couldn't read manifest file %s", boshManifestPath)
	}

	boshManifest, err := LoadYAML(resolvedYML)
	if err != nil {
		return errors.Wrapf(err, "failed to load BOSH deployment manifest %s", boshManifestPath)
	}

	currentInstanceGroup, ok := boshManifest.InstanceGroups.InstanceGroupByName(instanceGroupName)
	if !ok {
		return errors.Wrapf(err, "instance group %s not found in BOSH deployment manifest %s", instanceGroupName, boshManifestPath)
	}
	currentInstanceGroup.Instances = replicas

	// Generate Job Instances Spec
	for jobIdx, job := range currentInstanceGroup.Jobs {
		// Generate instance spec for each ig instance
		// This will be stored inside the current job under
		// job.properties.quarks
		jobsInstances := currentInstanceGroup.jobInstances(job.Name, initialRollout)

		// set jobs.properties.quarks.instances with the ig instances
		currentInstanceGroup.Jobs[jobIdx].Properties.Quarks.Instances = jobsInstances
	}

	// Run all pre-render scripts first.
	if err := runPreRenderScripts(currentInstanceGroup); err != nil {
		return err
	}

	jobGroup := errgroup.Group{}
	// Render all files for all jobs included in this instance_group.
	for _, job := range currentInstanceGroup.Jobs {

		job := job // https://golang.org/doc/faq#closures_and_goroutines
		jobGroup.Go(func() error {
			jobSpec, err := job.loadSpec(jobsDir)

			if err != nil {
				return errors.Wrapf(err, "failed to load job spec file %s for instance group %s", job.Name, instanceGroupName)
			}

			// Find job instance that's being rendered
			var currentJobInstance *JobInstance
			for _, instance := range job.Properties.Quarks.Instances {
				if instance.Index == specIndex {
					currentJobInstance = &instance
					break
				}
			}
			if currentJobInstance == nil {
				return errors.Errorf("no job instance found for spec index '%d'", specIndex)
			}

			// Loop over templates for rendering them
			templateGroup := errgroup.Group{}
			jobSrcDir := job.specDir(jobsDir)
			for source, destination := range jobSpec.Templates {
				source := source // https://golang.org/doc/faq#closures_and_goroutines
				destination := destination
				templateGroup.Go(func() error {
					absDest := filepath.Join(jobsOutputDir, job.Name, destination)
					err := os.MkdirAll(filepath.Dir(absDest), 0755)
					if err != nil {
						return err
					}

					properties := job.Properties.ToMap()

					renderPointer := btg.NewERBRenderer(
						&btg.EvaluationContext{
							Properties: properties,
						},

						&btg.InstanceInfo{
							Address:   currentJobInstance.Address,
							AZ:        currentJobInstance.AZ,
							Bootstrap: currentJobInstance.Bootstrap,
							ID:        currentJobInstance.ID,
							Index:     currentJobInstance.Index,
							IP:        podIP.String(),
							Name:      currentJobInstance.Name,
						},

						filepath.Join(jobSrcDir, JobSpecFilename),
					)

					// Create the destination file
					absDestFile, err := os.Create(absDest)
					if err != nil {
						return err
					}
					if err = renderPointer.Render(filepath.Join(jobSrcDir, "templates", source), absDestFile.Name()); err != nil {
						return err
					}
					if err = absDestFile.Close(); err != nil {
						return err
					}

					return nil
				})
			}

			return templateGroup.Wait()
		})
	}
	return jobGroup.Wait()
}

func runRenderScript(
	jobName string,
	scriptType string,
	scripts []string,
	igName string,
) error {
	for idx, script := range scripts {
		createErr := func(err error) error {
			return errors.Wrapf(err, "failed to run %s pre-render script %d, for job %s inside instance group %s", scriptType, idx, jobName, igName)
		}

		// Save the script to a temporary location.
		tmpFile, err := ioutil.TempFile(os.TempDir(), "script-")
		if err != nil {
			return createErr(err)
		}
		defer os.Remove(tmpFile.Name())

		// Write the pre-render script contents.
		if _, err = tmpFile.Write([]byte(script)); err != nil {
			return createErr(err)
		}

		if err = tmpFile.Close(); err != nil {
			return createErr(err)
		}

		// Run the pre-render script.
		cmd := exec.Command("/bin/bash", tmpFile.Name())

		errReader, err := cmd.StderrPipe()
		if err != nil {
			return createErr(err)
		}
		outReader, err := cmd.StdoutPipe()
		if err != nil {
			return createErr(err)
		}

		var outBuffer, errBuffer strings.Builder

		errScanner := bufio.NewScanner(errReader)
		go func() {
			for errScanner.Scan() {
				fmt.Fprintf(&errBuffer, "%s\n", errScanner.Text())
			}
		}()

		outScanner := bufio.NewScanner(outReader)
		go func() {
			for outScanner.Scan() {
				fmt.Fprintf(&outBuffer, "%s\n", outScanner.Text())
			}
		}()

		if err := cmd.Run(); err != nil {
			return createErr(errors.Wrapf(err, "stdout:\n%s\n\nstderr:\n%s", outBuffer.String(), errBuffer.String()))
		}
	}
	return nil
}

func runPreRenderScripts(instanceGroup *InstanceGroup) error {
	errGroup := errgroup.Group{}

	for _, job := range instanceGroup.Jobs {
		job := job // https://golang.org/doc/faq#closures_and_goroutines
		errGroup.Go(func() error {
			jobScripts := job.Properties.Quarks.PreRenderScripts

			if len(jobScripts.BPM) > 0 {
				if err := runRenderScript(job.Name, typeBPM, jobScripts.BPM, instanceGroup.Name); err != nil {
					return err
				}
			}
			if len(jobScripts.IgResolver) > 0 {
				if err := runRenderScript(job.Name, typeIGResolver, jobScripts.IgResolver, instanceGroup.Name); err != nil {
					return err
				}
			}
			if len(jobScripts.Jobs) > 0 {
				if err := runRenderScript(job.Name, typeJobs, jobScripts.Jobs, instanceGroup.Name); err != nil {
					return err
				}
			}

			return nil
		})
	}

	return errGroup.Wait()
}
