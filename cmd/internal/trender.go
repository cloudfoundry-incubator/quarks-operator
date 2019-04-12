package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	btg "github.com/viovanov/bosh-template-go"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

// templateRenderCmd represents the template-render command
var templateRenderCmd = &cobra.Command{
	Use:   "template-render [flags]",
	Short: "Renders a bosh manifest",
	Long: `Renders a bosh manifest.

This will render a provided manifest instance-group
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		boshManifestPath := viper.GetString("bosh-manifest-path")
		jobsDir := viper.GetString("jobs-dir")
		instanceGroupName := viper.GetString("instance-group-name")

		specIndex := viper.GetInt("spec-index")
		if specIndex < 0 {
			// calculate index following the formula specified in
			// docs/rendering_templates.md
			azIndex := viper.GetInt("az-index")
			if azIndex < 0 {
				return fmt.Errorf("required parameter 'az-index' not set")
			}
			replicas := viper.GetInt("replicas")
			if replicas < 0 {
				return fmt.Errorf("required parameter 'replicas' not set")
			}
			podOrdinal := viper.GetInt("pod-ordinal")
			if podOrdinal < 0 {
				// Infer ordinal from hostname
				hostname, err := os.Hostname()
				r, _ := regexp.Compile(`-(\d+)(-|\.|\z)`)
				if err != nil {
					return errors.Wrap(err, "getting the hostname")
				}
				match := r.FindStringSubmatch(hostname)
				if len(match) < 2 {
					return fmt.Errorf("can not extract the pod ordinal from hostname '%s'", hostname)
				}
				podOrdinal, _ = strconv.Atoi(match[1])
			}

			specIndex = (azIndex-1)*replicas + podOrdinal
		}

		return RenderData(boshManifestPath, jobsDir, "/var/vcap/jobs/", instanceGroupName, specIndex)
	},
}

func init() {
	rootCmd.AddCommand(templateRenderCmd)

	templateRenderCmd.Flags().StringP("bosh-manifest-path", "m", "", "path to the bosh manifest file")
	templateRenderCmd.Flags().StringP("jobs-dir", "j", "", "path to the jobs dir.")
	templateRenderCmd.Flags().StringP("instance-group-name", "g", "", "the instance-group name to render")
	templateRenderCmd.Flags().IntP("spec-index", "", -1, "index of the instance spec")
	templateRenderCmd.Flags().IntP("az-index", "", -1, "az index")
	templateRenderCmd.Flags().IntP("pod-ordinal", "", -1, "pod ordinal")
	templateRenderCmd.Flags().IntP("replicas", "", -1, "number of replicas")

	viper.BindPFlag("bosh-manifest-path", templateRenderCmd.Flags().Lookup("bosh-manifest-path"))
	viper.BindPFlag("jobs-dir", templateRenderCmd.Flags().Lookup("jobs-dir"))
	viper.BindPFlag("instance-group-name", templateRenderCmd.Flags().Lookup("instance-group-name"))
	viper.BindPFlag("az-index", templateRenderCmd.Flags().Lookup("az-index"))
	viper.BindPFlag("spec-index", templateRenderCmd.Flags().Lookup("spec-index"))
	viper.BindPFlag("pod-ordinal", templateRenderCmd.Flags().Lookup("pod-ordinal"))
	viper.BindPFlag("replicas", templateRenderCmd.Flags().Lookup("replicas"))

	argToEnv["bosh-manifest-path"] = "BOSH_MANIFEST_PATH"
	argToEnv["jobs-dir"] = "JOBS_DIR"
	argToEnv["instance-group-name"] = "INSTANCE_GROUP_NAME"
	argToEnv["docker-image-repository"] = "DOCKER_IMAGE_REPOSITORY"
	argToEnv["spec-index"] = "SPEC_INDEX"
	argToEnv["az-index"] = "AZ_INDEX"
	argToEnv["pod-ordinal"] = "POD_ORDINAL"
	argToEnv["replicas"] = "REPLICAS"

	for arg, env := range argToEnv {
		viper.BindEnv(arg, env)
	}
}

// RenderData will render manifest instance group
func RenderData(boshManifestPath string, jobsDir string, jobsOutputDir string, instanceGroupName string, specIndex int) error {

	// Loading deployment manifest file
	resolvedYML, err := ioutil.ReadFile(boshManifestPath)
	if err != nil {
		return errors.Wrapf(err, "couldn't read manifest file %s", boshManifestPath)
	}
	boshManifest := manifest.Manifest{}
	err = yaml.Unmarshal(resolvedYML, &boshManifest)
	if err != nil {
		return errors.Wrapf(err, "failed to unmarshal deployment manifest %s", boshManifestPath)
	}

	// Loop over instancegroups
	for _, instanceGroup := range boshManifest.InstanceGroups {

		// Filter based on the instance group name
		if instanceGroup.Name != instanceGroupName {
			continue
		}

		// Render all files for all jobs included in this instance_group.
		for _, job := range instanceGroup.Jobs {
			jobInstanceLinks := []manifest.Link{}

			// Find job instance that's being rendered
			var currentJobInstance *manifest.JobInstance
			for _, instance := range job.Properties.BOSHContainerization.Instances {
				if instance.Index == specIndex {
					currentJobInstance = &instance
					break
				}
			}
			if currentJobInstance == nil {
				return fmt.Errorf("no instance found for spec index '%d'", specIndex)
			}

			// Loop over name and link
			for name, jobConsumersLink := range job.Properties.BOSHContainerization.Consumes {
				jobInstances := []manifest.JobInstance{}

				// Loop over instances of link
				for _, jobConsumerLinkInstance := range jobConsumersLink.Instances {
					jobInstances = append(jobInstances, manifest.JobInstance{
						Address: jobConsumerLinkInstance.Address,
						AZ:      jobConsumerLinkInstance.AZ,
						ID:      jobConsumerLinkInstance.ID,
						Index:   jobConsumerLinkInstance.Index,
						Name:    jobConsumerLinkInstance.Name,
					})
				}

				jobInstanceLinks = append(jobInstanceLinks, manifest.Link{
					Name:       name,
					Instances:  jobInstances,
					Properties: jobConsumersLink.Properties,
				})
			}

			jobSrcDir := filepath.Join(jobsDir, "jobs-src", job.Release, job.Name)
			jobMFFile := filepath.Join(jobSrcDir, "job.MF")
			jobMfBytes, err := ioutil.ReadFile(jobMFFile)
			if err != nil {
				return errors.Wrapf(err, "failed to read job spec file %s", jobMFFile)
			}

			jobSpec := manifest.JobSpec{}
			if err := yaml.Unmarshal([]byte(jobMfBytes), &jobSpec); err != nil {
				return errors.Wrapf(err, "failed to unmarshal job spec %s", jobMFFile)
			}

			// Loop over templates for rendering files
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

					jobMFFile,
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
