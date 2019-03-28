package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

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

		deploymentManifest := viper.GetString("manifest")
		jobsDir := viper.GetString("jobs_dir")
		instanceGroupName := viper.GetString("instance_group_name")
		address := viper.GetString("address")
		az := viper.GetString("az")
		if len(az) == 0 {
			return fmt.Errorf("az cannot be empty")
		}
		id := viper.GetString("id")
		index := viper.GetString("index")
		if len(index) == 0 {
			// calculate index
			azIndex, err := strconv.Atoi(os.Getenv("CF_OPERATOR_AZ_INDEX"))
			if err != nil {
				return err
			}
			podIndex, err := strconv.Atoi(os.Getenv("POD_INDEX"))
			if err != nil {
				return fmt.Errorf("podIndex not found")
			}
			index = strconv.Itoa(podIndex * azIndex)
		}
		ip := viper.GetString("ip")
		if len(ip) == 0 {
			return fmt.Errorf("ip cannot be empty")
		}
		name := viper.GetString("name")

		network := map[string]interface{}{ // TODO Do I need to create struct for this ?
			"default": map[string]interface{}{
				"ip":              ip,
				"dns_record_name": address,
			},
		}

		indexInt, err := strconv.Atoi(index)
		if err != nil {
			return fmt.Errorf("index Atoi failed")
		}

		jobInstance := manifest.JobInstance{ // TODO do we need deployment
			Address: address,
			AZ:      az,
			ID:      id,
			Index:   indexInt,
			Name:    name,
			IP:      ip,
			Network: network,
		}

		return RenderData(deploymentManifest, jobsDir, instanceGroupName, jobInstance)
	},
}

func init() {
	rootCmd.AddCommand(templateRenderCmd)

	templateRenderCmd.Flags().StringP("manifest", "f", "", "path to the manifest file")
	templateRenderCmd.MarkFlagRequired("manifest")
	templateRenderCmd.Flags().StringP("jobs-dir", "j", "", "path to the jobs dir.")
	templateRenderCmd.MarkFlagRequired("jobs-dir")
	templateRenderCmd.Flags().StringP("instance-group-name", "g", "", "the instance-group name to render")
	templateRenderCmd.Flags().StringP("address", "a", "", "address of the instance spec")
	templateRenderCmd.MarkFlagRequired("address")
	templateRenderCmd.Flags().StringP("az", "z", "", "az's of the instance spec")
	templateRenderCmd.Flags().StringP("id", "d", "", "id of the instance spec")
	templateRenderCmd.MarkFlagRequired("id")
	templateRenderCmd.Flags().StringP("index", "x", "", "index of the instance spec")
	templateRenderCmd.Flags().StringP("ip", "i", "", "ip of the instance spec")
	templateRenderCmd.Flags().StringP("name", "m", "", "name of the instance spec")
	templateRenderCmd.MarkFlagRequired("name")

	viper.AutomaticEnv()
	viper.BindPFlag("jobs_dir", templateRenderCmd.Flags().Lookup("jobs-dir"))
	viper.BindPFlag("instance_group_name", templateRenderCmd.Flags().Lookup("instance-group-name"))
	viper.BindPFlag("address", templateRenderCmd.Flags().Lookup("address"))
	viper.BindPFlag("az", templateRenderCmd.Flags().Lookup("az"))
	viper.BindPFlag("id", templateRenderCmd.Flags().Lookup("id"))
	viper.BindPFlag("index", templateRenderCmd.Flags().Lookup("index"))
	viper.BindPFlag("name", templateRenderCmd.Flags().Lookup("name"))
	viper.BindPFlag("ip", templateRenderCmd.Flags().Lookup("ip"))

	viper.BindEnv("az", "CF_OPERATOR_AZ")
	viper.BindEnv("ip", "POD_IP")

}

// RenderData will render manifest instance group
func RenderData(deploymentManifest string, jobsDir string, instanceGroupName string, jobInstance manifest.JobInstance) error {

	// Loading deployment manifest file
	resolvedYML, err := ioutil.ReadFile(deploymentManifest)
	if err != nil {
		return err
	}
	manifestDeployment := manifest.Manifest{}
	err = yaml.Unmarshal(resolvedYML, &manifestDeployment)
	if err != nil {
		return err
	}

	// Loop over instancegroups
	for _, instanceGroup := range manifestDeployment.InstanceGroups {

		// Filter based on the instance group name
		if instanceGroup.Name != instanceGroupName {
			continue
		}

		// Render all files for all jobs included in this instance_group.
		for _, job := range instanceGroup.Jobs {
			jobInstanceLinks := []manifest.Link{}

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
				return err
			}

			jobSpec := manifest.JobSpec{}
			if err := yaml.Unmarshal([]byte(jobMfBytes), &jobSpec); err != nil {
				return err
			}

			// Loop over templates for rendering files
			for source, destination := range jobSpec.Templates {
				absDest := filepath.Join(jobsDir, job.Name, destination)
				os.MkdirAll(filepath.Dir(absDest), 0755)

				properties := job.Properties.ToMap()

				renderPointer := btg.NewERBRenderer(
					&btg.EvaluationContext{
						Properties: properties,
					},

					&btg.InstanceInfo{
						Address: jobInstance.Address,
						AZ:      jobInstance.AZ,
						ID:      jobInstance.ID,
						Index:   string(jobInstance.Index),
						Name:    jobInstance.Name,
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
