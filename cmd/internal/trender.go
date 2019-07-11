package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

var hostnameRegex = regexp.MustCompile(`-(\d+)(-|\.|\z)`)

// templateRenderCmd is the template-render command.
var templateRenderCmd = &cobra.Command{
	Use:   "template-render [flags]",
	Short: "Renders a bosh manifest",
	Long: `Renders a bosh manifest.

This will render a provided manifest instance-group
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		boshManifestPath := viper.GetString("bosh-manifest-path")
		jobsDir := viper.GetString("jobs-dir")
		outputDir := viper.GetString("output-dir")

		instanceGroupName := viper.GetString("instance-group-name")
		if len(instanceGroupName) == 0 {
			return fmt.Errorf("instance-group-name cannot be empty")
		}

		specIndex := viper.GetInt("spec-index")
		if specIndex < 0 {
			// Calculate index following the formula specified in
			// docs/rendering_templates.md.
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
				// Infer ordinal from hostname.
				hostname, err := os.Hostname()
				if err != nil {
					return err
				}
				match := hostnameRegex.FindStringSubmatch(hostname)
				if len(match) < 2 {
					return fmt.Errorf("cannot extract the pod ordinal from hostname '%s'", hostname)
				}
				podOrdinal, err = strconv.Atoi(match[1])
				if err != nil {
					return err
				}
			}

			specIndex = (azIndex-1)*replicas + podOrdinal
		}

		return manifest.RenderJobTemplates(boshManifestPath, jobsDir, outputDir, instanceGroupName, specIndex)
	},
}

func init() {
	utilCmd.AddCommand(templateRenderCmd)

	templateRenderCmd.Flags().StringP("jobs-dir", "j", "", "path to the jobs dir.")
	templateRenderCmd.Flags().StringP("output-dir", "d", manifest.VolumeJobsDirMountPath, "path to output dir.")
	templateRenderCmd.Flags().IntP("spec-index", "", -1, "index of the instance spec")
	templateRenderCmd.Flags().IntP("az-index", "", -1, "az index")
	templateRenderCmd.Flags().IntP("pod-ordinal", "", -1, "pod ordinal")
	templateRenderCmd.Flags().IntP("replicas", "", -1, "number of replicas")

	viper.BindPFlag("jobs-dir", templateRenderCmd.Flags().Lookup("jobs-dir"))
	viper.BindPFlag("output-dir", templateRenderCmd.Flags().Lookup("output-dir"))
	viper.BindPFlag("az-index", templateRenderCmd.Flags().Lookup("az-index"))
	viper.BindPFlag("spec-index", templateRenderCmd.Flags().Lookup("spec-index"))
	viper.BindPFlag("pod-ordinal", templateRenderCmd.Flags().Lookup("pod-ordinal"))
	viper.BindPFlag("replicas", templateRenderCmd.Flags().Lookup("replicas"))

	argToEnv := map[string]string{
		"jobs-dir":                "JOBS_DIR",
		"output-dir":              "OUTPUT_DIR",
		"docker-image-repository": "DOCKER_IMAGE_REPOSITORY",
		"spec-index":              "SPEC_INDEX",
		"az-index":                "AZ_INDEX",
		"pod-ordinal":             "POD_ORDINAL",
		"replicas":                "REPLICAS",
	}
	AddEnvToUsage(templateRenderCmd, argToEnv)
}
