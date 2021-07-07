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

	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

const (
	typeBPM        = "bpm"
	typeIGResolver = "ig_resolver"
	typeJobs       = "jobs"
)

// RenderJobTemplates will render templates for all jobs of the instance group
// https://bosh.io/docs/create-release/#job-specs
// boshManifest is a resolved manifest for a single instance group
//
// qsts pod mutator sets pod-ordinal (to name suffix of the pod)
// replicas is set to 1 in the container factory
// azIndex is set to 1 in the container factory
// qsts controller overwrites replicas, if InjectReplicasEnv is true, otherwise replicas is 1.
// qsts controller overwrites azIndex (1..n), or to 0 if ig.AZs is null
func RenderJobTemplates(
	boshManifestPath string,
	jobsDir string,
	jobsOutputDir string,
	instanceGroupName string,
	podIP net.IP,
	azIndex int,
	podOrdinal int,
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

	// adapt replicas to always be valid
	if podOrdinal+1 > replicas {
		replicas = podOrdinal + 1
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

	ig, ok := boshManifest.InstanceGroups.InstanceGroupByName(instanceGroupName)
	if !ok {
		return errors.Wrapf(err, "instance group %s not found in BOSH deployment manifest %s", instanceGroupName, boshManifestPath)
	}
	ig.Instances = replicas

	// Make sure azIndex is at least 1 and within bounds, if any ig.AZs is configured
	// It has to be 0, if no AZs are configured.
	if err := validateAZ(len(ig.AZs), azIndex); err != nil {
		return errors.Wrap(err, "az index doesn't match azs")
	}

	// this is needed for old integration tests to pass, should be ok since
	// specIndex is the same for 0 and 1
	if len(ig.AZs) == 0 && azIndex == 1 {
		azIndex = 0
	}

	// Generate Job Instances Spec
	for jobIdx, job := range ig.Jobs {
		// Generate instance spec for each ig instance, so templates
		// can access all instance specs.
		jobInstances := ig.newJobInstances(job.Name, initialRollout)
		ig.Jobs[jobIdx].Properties.Quarks.Instances = jobInstances
	}

	// Run all pre-render scripts first.
	if err := runPreRenderScripts(ig); err != nil {
		return err
	}

	// We use a very large value as a maximum number of replicas per instance group, per AZ.
	// We do this in lieu of using the actual replica count, which would cause pods to always restart.
	// azindex podOrdinal  specIndex
	//    0       0          0
	//    0       1          1
	//    1       0          0
	//    1       1          1
	//    2       0          10000
	//    2       1          10001
	specIndex := names.SpecIndex(azIndex, podOrdinal)

	// Render all files for all jobs included in this instance_group in parallel.
	jobGroup := errgroup.Group{}
	for _, job := range ig.Jobs {

		job := job // https://golang.org/doc/faq#closures_and_goroutines
		jobGroup.Go(func() error {
			jobSpec, err := job.loadSpec(jobsDir)

			if err != nil {
				return errors.Wrapf(err, "failed to load job spec file %s for instance group %s", job.Name, instanceGroupName)
			}

			// Find job instance that's being rendered
			currentJobInstance := job.Properties.Quarks.jobInstance(ig.AZs, azIndex, specIndex)
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

// Verify that he azIndex, which is starting at 1, matches the number of AZs.
// An index of 0 is used if no AZs are configured.
// azs: 0, idx: 0
// azs: 1, idx: 1
// azs: 2, idx: 1,2
// azs: 3, idx: 1,2,3
func validateAZ(azMax int, idx int) error {
	// idx 0 is only allowed if no azs
	if azMax == 0 && idx == 0 {
		return nil
	}
	// idx must be within azMax
	if azMax >= 1 && idx >= 1 && idx <= azMax {
		return nil
	}
	// the azindex will later be set to 0
	if azMax == 0 && idx == 1 {
		return nil
	}
	return errors.Errorf("%d <= %d", idx, azMax)
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
