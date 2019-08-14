package cmd

import (
	"net"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

const (
	tRenderFailedMessage = "template-render command failed."
)

var hostnameRegex = regexp.MustCompile(`-(\d+)(-|\.|\z)`)

// templateRenderCmd is the template-render command.
var templateRenderCmd = &cobra.Command{
	Use:   "template-render [flags]",
	Short: "Renders a bosh manifest",
	Long: `Renders a bosh manifest.

This will render a provided manifest instance-group
`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if err != nil {
				time.Sleep(debugGracePeriod)
			}
		}()

		boshManifestPath := viper.GetString("bosh-manifest-path")
		jobsDir := viper.GetString("jobs-dir")
		outputDir := viper.GetString("output-dir")

		instanceGroupName := viper.GetString("instance-group-name")
		if len(instanceGroupName) == 0 {
			return errors.Errorf("%s instance-group-name flag is empty.", tRenderFailedMessage)
		}

		specIndex := viper.GetInt("spec-index")
		if specIndex < 0 {
			// Calculate index following the formula specified in
			// docs/rendering_templates.md.
			azIndex := viper.GetInt("az-index")
			if azIndex < 0 {
				return errors.Errorf("%s az-index is negative. %d", tRenderFailedMessage, azIndex)
			}
			replicas := viper.GetInt("replicas")
			if replicas < 0 {
				return errors.Errorf("%s replicas flag is empty.", tRenderFailedMessage)
			}
			podOrdinal := viper.GetInt("pod-ordinal")
			if podOrdinal < 0 {
				// Infer ordinal from hostname.
				hostname, err := os.Hostname()
				if err != nil {
					return errors.Wrapf(err, "%s Failed to get hostname from os.Hostname()", tRenderFailedMessage)
				}
				match := hostnameRegex.FindStringSubmatch(hostname)
				if len(match) < 2 {
					return errors.Errorf("%s Cannot extract podOrdinal flag value from hostname %s", tRenderFailedMessage, hostname)
				}
				podOrdinal, err = strconv.Atoi(match[1])
				if err != nil {
					return errors.Wrapf(err, "%s String to int conversion failed from hostname for calculatinf podOrdinal flag value %s", tRenderFailedMessage, hostname)
				}
			}
			specIndex = (azIndex-1)*replicas + podOrdinal
		}

		podName := viper.GetString("pod-name")
		podIP := net.ParseIP(viper.GetString("pod-ip"))

		return manifest.RenderJobTemplates(boshManifestPath, jobsDir, outputDir, instanceGroupName, specIndex, podName, podIP)
	},
}

func init() {
	utilCmd.AddCommand(templateRenderCmd)

	templateRenderCmd.Flags().StringP("jobs-dir", "j", "", "path to the jobs dir.")
	templateRenderCmd.Flags().StringP("output-dir", "d", converter.VolumeJobsDirMountPath, "path to output dir.")
	templateRenderCmd.Flags().IntP("spec-index", "", -1, "index of the instance spec")
	templateRenderCmd.Flags().IntP("az-index", "", -1, "az index")
	templateRenderCmd.Flags().IntP("pod-ordinal", "", -1, "pod ordinal")
	templateRenderCmd.Flags().IntP("replicas", "", -1, "number of replicas")
	templateRenderCmd.Flags().StringP("pod-name", "", "", "pod name")
	templateRenderCmd.Flags().StringP("pod-ip", "", "", "pod IP")

	viper.BindPFlag("jobs-dir", templateRenderCmd.Flags().Lookup("jobs-dir"))
	viper.BindPFlag("output-dir", templateRenderCmd.Flags().Lookup("output-dir"))
	viper.BindPFlag("az-index", templateRenderCmd.Flags().Lookup("az-index"))
	viper.BindPFlag("spec-index", templateRenderCmd.Flags().Lookup("spec-index"))
	viper.BindPFlag("pod-ordinal", templateRenderCmd.Flags().Lookup("pod-ordinal"))
	viper.BindPFlag("replicas", templateRenderCmd.Flags().Lookup("replicas"))
	viper.BindPFlag("pod-name", templateRenderCmd.Flags().Lookup("pod-name"))
	viper.BindPFlag("pod-ip", templateRenderCmd.Flags().Lookup("pod-ip"))

	argToEnv := map[string]string{
		"jobs-dir":                "JOBS_DIR",
		"output-dir":              "OUTPUT_DIR",
		"docker-image-repository": "DOCKER_IMAGE_REPOSITORY",
		"spec-index":              "SPEC_INDEX",
		"az-index":                "AZ_INDEX",
		"pod-ordinal":             "POD_ORDINAL",
		"replicas":                "REPLICAS",
		"pod-name":                converter.PodNameEnvVar,
		"pod-ip":                  converter.PodIPEnvVar,
	}
	AddEnvToUsage(templateRenderCmd, argToEnv)
}
