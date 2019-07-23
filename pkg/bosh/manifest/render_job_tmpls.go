package manifest

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pkg/errors"
	btg "github.com/viovanov/bosh-template-go"
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
) error {

	if podIP == nil {
		return fmt.Errorf("the pod IP is empty")
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

			// Run pre-render scripts for the current job.
			for idx, script := range job.Properties.BOSHContainerization.PreRenderScripts {
				if err := runPreRenderScript(script, idx, false); err != nil {
					return errors.Wrapf(err, "failed to run pre-render script %d for job %s", idx, job.Name)
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
						Address:   currentJobInstance.Address,
						AZ:        currentJobInstance.AZ,
						Bootstrap: currentJobInstance.Index == 0,
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
				defer absDestFile.Close()
				if err = renderPointer.Render(filepath.Join(jobSrcDir, "templates", source), absDestFile.Name()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func runPreRenderScript(script string, idx int, silent bool) error {
	// Save the script to a temporary location.
	tmpFile, err := ioutil.TempFile(os.TempDir(), "script-")
	if err != nil {
		return errors.Wrap(err, "failed to create a temp file for the pre-render script")
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write the pre-render script contents.
	if _, err = tmpFile.Write([]byte(script)); err != nil {
		return errors.Wrap(err, "failed to write pre-render script to file")
	}

	// Run the pre-render script.
	cmd := exec.Command("/bin/bash", tmpFile.Name())

	if !silent {
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
				fmt.Printf("pre-render-err-%d | %s\n", idx, errScanner.Text())
			}
		}()

		outScanner := bufio.NewScanner(outReader)
		go func() {
			for outScanner.Scan() {
				fmt.Printf("pre-render-out-%d | %s\n", idx, outScanner.Text())
			}
		}()
	}

	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "failed to run pre-render script %d", idx)
	}

	return nil
}
