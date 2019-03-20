package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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
		_ = viper.GetString("bootstrap") // TODO do we require this ?
		id := viper.GetString("id")
		index := viper.GetInt("index")
		ip := viper.GetString("ip")
		name := viper.GetString("name")

		network := map[string]interface{}{
			"default": map[string]interface{}{
				"ip":              ip,
				"dns_record_name": address,
			},
		}

		jobInstance := manifest.JobInstance{ // TODO do we need deployment
			Address:   address,
			AZ:        az,
			ID:        id,
			Bootstrap: index,
			Index:     index,
			Name:      name,
			IP:        ip,
			Network:   network,
		}

		return RenderData(deploymentManifest, jobsDir, instanceGroupName, jobInstance)
	},
}

func init() {
	rootCmd.AddCommand(templateRenderCmd)

	templateRenderCmd.Flags().StringP("manifest", "f", "", "path to the manifest file")
	templateRenderCmd.MarkFlagRequired("manifest")
	templateRenderCmd.Flags().StringP("jobs-dir", "j", "", "path to the jobs dir. The flag can be specify multiple times")
	templateRenderCmd.MarkFlagRequired("jobs-dir")
	templateRenderCmd.Flags().StringSliceP("instance-groups", "g", []string{}, "the instance-groups filter")
	templateRenderCmd.Flags().StringP("address", "a", "", "address of the instance spec")
	templateRenderCmd.MarkFlagRequired("address")
	templateRenderCmd.Flags().StringP("az", "z", "", "az of the instance spec")
	templateRenderCmd.MarkFlagRequired("az")
	templateRenderCmd.Flags().StringP("bootstrap", "s", "", "bootstrap of the instance spec")
	templateRenderCmd.MarkFlagRequired("bootstrap")
	templateRenderCmd.Flags().StringP("id", "d", "", "id of the instance spec")
	templateRenderCmd.MarkFlagRequired("id")
	templateRenderCmd.Flags().StringP("index", "x", "", "index of the instance spec")
	templateRenderCmd.MarkFlagRequired("index")
	templateRenderCmd.Flags().StringP("ip", "i", "", "ip of the instance spec")
	templateRenderCmd.MarkFlagRequired("ip")
	templateRenderCmd.Flags().StringP("name", "m", "", "name of the instance spec")
	templateRenderCmd.MarkFlagRequired("name")

	viper.AutomaticEnv()
	viper.BindPFlag("jobs_dir", templateRenderCmd.Flags().Lookup("jobs-dir"))
	viper.BindPFlag("instance_group_name", templateRenderCmd.Flags().Lookup("instance-group_name"))
	viper.BindPFlag("address", templateRenderCmd.Flags().Lookup("address"))
	viper.BindPFlag("az", templateRenderCmd.Flags().Lookup("az"))
	viper.BindPFlag("bootstrap", templateRenderCmd.Flags().Lookup("bootstrap"))
	viper.BindPFlag("id", templateRenderCmd.Flags().Lookup("id"))
	viper.BindPFlag("index", templateRenderCmd.Flags().Lookup("index"))
	viper.BindPFlag("name", templateRenderCmd.Flags().Lookup("name"))
	viper.BindPFlag("ip", templateRenderCmd.Flags().Lookup("ip"))
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

			// Make sure that job.properties.bosh_containerization exists
			if _, ok := job.Properties["bosh_containerization"]; !ok {
				job.Properties["bosh_containerization"] = map[interface{}]interface{}{}
			}

			// Make sure that job.properties.bosh_containerization.consumes exists
			if _, ok := job.Properties["bosh_containerization"].(map[interface{}]interface{})["consumes"]; !ok {
				job.Properties["bosh_containerization"].(map[interface{}]interface{})["consumes"] = map[interface{}]interface{}{}
			}

			// Loop over name and link
			for name := range job.Properties["bosh_containerization"].(map[interface{}]interface{})["consumes"].(map[interface{}]interface{}) {
				jobInstances := []manifest.JobInstance{}

				jobConsumersLink, err := job.GetJobBoshContainerizationConsumers(name.(string))
				if err != nil {
					return err
				}

				// Loop over instances of link
				for _, jobConsumerLinkInstance := range jobConsumersLink.Instances {
					jobInstances = append(jobInstances, manifest.JobInstance{
						Address:   jobConsumerLinkInstance.Address,
						AZ:        jobConsumerLinkInstance.AZ,
						ID:        jobConsumerLinkInstance.ID,
						Index:     jobConsumerLinkInstance.Index,
						Name:      jobConsumerLinkInstance.Name,
						Bootstrap: jobConsumerLinkInstance.Bootstrap,
					})
				}

				jobInstanceLinks = append(jobInstanceLinks, manifest.Link{
					Name:       name.(string),
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

			for source, destination := range jobSpec.Templates {
				absDest := filepath.Join(jobsDir, job.Name, destination)
				os.MkdirAll(filepath.Dir(absDest), 0777) // TODO what is the right one ?
				renderPointer := btg.NewERBRenderer(
					&btg.EvaluationContext{
						Properties: job.Properties,
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

				absDestFile, err := os.Create(absDest)
				if err != nil {
					fmt.Println("New error")
					return err
				}
				defer absDestFile.Close()
				fmt.Println(absDestFile.Name())
				if err = renderPointer.Render(filepath.Join(jobsDir, "templates", source), absDestFile.Name()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// contains filter instance groups based on the name // TODO check if you can use from dgather
func contains(igList []string, name string) bool {
	for _, igName := range igList {
		if name == igName {
			return true
		}
	}
	return false
}
