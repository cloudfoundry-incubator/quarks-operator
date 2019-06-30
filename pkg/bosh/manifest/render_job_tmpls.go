package manifest

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	btg "github.com/viovanov/bosh-template-go"
)

// RenderJobTemplates will render templates for all jobs of the instance group
// https://bosh.io/docs/create-release/#job-specs
// boshManifest is a resolved manifest for a single instance group
func RenderJobTemplates(boshManifestPath string, jobsDir string, jobsOutputDir string, instanceGroupName string, specIndex int) error {

	// Loading deployment manifest file
	resolvedYML, err := ioutil.ReadFile(boshManifestPath)
	if err != nil {
		return errors.Wrapf(err, "couldn't read manifest file %s", boshManifestPath)
	}
	boshManifest, err := LoadYAML(resolvedYML)
	if err != nil {
		return errors.Wrapf(err, "failed to load BOSH deployment manifest %s", boshManifestPath)
	}

	// Loop over instancegroups
	for _, instanceGroup := range boshManifest.InstanceGroups {

		// Filter based on the instance group name
		if instanceGroup.Name != instanceGroupName {
			continue
		}

		// Render all files for all jobs included in this instance_group.
		for _, job := range instanceGroup.Jobs {
			jobSpec, err := job.loadSpec(jobsDir)

			if err != nil {
				return errors.Wrapf(err, "failed to load job spec file %s", job.Name)
			}

			// Run patches for the current job
			for idx, patch := range job.Properties.BOSHContainerization.Patches {
				err := runPatch(patch, idx)
				if err != nil {
					return errors.Wrapf(err, "failed to run patch %d for job %s", idx, job.Name)
				}
			}

			// Find job instance that's being rendered
			var currentJobInstance *JobInstance
			for _, instance := range job.Properties.BOSHContainerization.Instances {
				if instance.Index == specIndex {
					currentJobInstance = &instance
					break
				}
			}
			if currentJobInstance == nil {
				return fmt.Errorf("no instance found for spec index '%d'", specIndex)
			}

			// Loop over templates for rendering files
			jobSrcDir := job.specDir(jobsDir)
			for source, destination := range jobSpec.Templates {
				absDest := filepath.Join(jobsOutputDir, job.Name, destination)
				os.MkdirAll(filepath.Dir(absDest), 0755)

				properties := job.Properties.ToMap()

				renderPointer := btg.NewERBRenderer(
					&btg.EvaluationContext{
						Properties: properties,
					},

					&btg.InstanceInfo{
						Address: currentJobInstance.Address,
						AZ:      currentJobInstance.AZ,
						ID:      currentJobInstance.ID,
						Index:   string(currentJobInstance.Index),
						Name:    currentJobInstance.Name,
					},

					filepath.Join(jobSrcDir, JobSpecFilename),
				)

				// Create the destination file
				absDestFile, err := os.Create(absDest)
				if err != nil {
					return err
				}
				defer absDestFile.Close()
				if err = renderPointer.Render(filepath.Join(jobSrcDir, "templates", source), absDestFile.Name()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func runPatch(patch string, idx int) error {
	// Save the patch to a temporary location
	tmpFile, err := ioutil.TempFile(os.TempDir(), "patch-")
	if err != nil {
		return errors.Wrap(err, "failed to create a temp file for a patch")
	}
	// Cleanup once we're done
	defer os.Remove(tmpFile.Name())
	// Write patch contents
	if _, err = tmpFile.Write([]byte(patch)); err != nil {
		return errors.Wrap(err, "failed to write patch to file")
	}
	// Close the patch file
	if err := tmpFile.Close(); err != nil {
		return errors.Wrap(err, "failed to close patch file")
	}

	// Run the patch
	cmd := exec.Command("/bin/bash", tmpFile.Name())

	errReader, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stderr pipe")
	}
	outReader, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe")
	}

	errScanner := bufio.NewScanner(errReader)
	go func() {
		for errScanner.Scan() {
			fmt.Printf("patch-err-%d | %s\n", idx, errScanner.Text())
		}
	}()

	outScanner := bufio.NewScanner(outReader)
	go func() {
		for outScanner.Scan() {
			fmt.Printf("patch-out-%d | %s\n", idx, outScanner.Text())
		}
	}()

	err = cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "failed to start patch script %d", idx)
	}

	err = cmd.Wait()
	if err != nil {
		return errors.Wrapf(err, "failed to wait for patch script %d", idx)
	}

	return nil
}
