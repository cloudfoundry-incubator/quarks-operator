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

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpmconverter"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
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
	PreRun: func(cmd *cobra.Command, args []string) {
		boshManifestFlagViperBind(cmd.Flags())
		instanceGroupFlagViperBind(cmd.Flags())
		initialRolloutFlagViperBind(cmd.Flags())
	},

	RunE: func(cmd *cobra.Command, args []string) (err error) {
		defer func() {
			if err != nil {
				time.Sleep(debugGracePeriod)
			}
		}()

		boshManifestPath, err := boshManifestFlagValidation()
		if err != nil {
			return errors.Wrap(err, tRenderFailedMessage)
		}

		jobsDir := viper.GetString("jobs-dir")
		outputDir := viper.GetString("output-dir")

		instanceGroupName, err := instanceGroupFlagValidation()
		if err != nil {
			return errors.Wrap(err, tRenderFailedMessage)
		}

		replicas := viper.GetInt("replicas")
		if replicas < 0 {
			// If the replicas count config is missing, it means the user has chosen to ignore the value
			// because no templates were using it, and to optimize scaling by avoiding unnecessary restarts.
			replicas = 1
		}

		podOrdinal := viper.GetInt("pod-ordinal")
		if podOrdinal < 0 {
			// If the pod ordinal is not explicitly set, infer ordinal from hostname.
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
				return errors.Wrapf(err, "%s String to int conversion failed from hostname for calculating podOrdinal flag value %s", tRenderFailedMessage, hostname)
			}
		}

		// Calculate index following the formula specified in
		// docs/rendering_templates.md.
		azIndex := viper.GetInt("az-index")
		if azIndex < 0 {
			return errors.Errorf("%s az-index is negative. %d", tRenderFailedMessage, azIndex)
		}

		// We use a very large value as a maximum number of replicas per instance group, per AZ
		// We do this in lieu of using the actual replica count, which would cause pods to always restart
		specIndex := names.SpecIndex(azIndex, podOrdinal)

		initialRollout := viper.GetBool("initial-rollout")
		podIP := net.ParseIP(viper.GetString("pod-ip"))

		if podOrdinal+1 > replicas {
			replicas = podOrdinal + 1
		}

		return manifest.RenderJobTemplates(boshManifestPath, jobsDir, outputDir, instanceGroupName, specIndex, podIP, replicas, initialRollout)
	},
}

func init() {
	pf := templateRenderCmd.Flags()
	utilCmd.AddCommand(templateRenderCmd)
	pf.StringP("jobs-dir", "j", "", "path to the jobs dir.")
	pf.StringP("output-dir", "d", bpmconverter.VolumeJobsDirMountPath, "path to output dir.")
	pf.IntP("az-index", "", -1, "az index")
	pf.IntP("pod-ordinal", "", -1, "pod ordinal")
	pf.IntP("replicas", "", -1, "number of replicas")
	pf.StringP("pod-ip", "", "", "pod IP")

	viper.BindPFlag("jobs-dir", pf.Lookup("jobs-dir"))
	viper.BindPFlag("output-dir", pf.Lookup("output-dir"))
	viper.BindPFlag("az-index", pf.Lookup("az-index"))
	viper.BindPFlag("pod-ordinal", pf.Lookup("pod-ordinal"))
	viper.BindPFlag("replicas", pf.Lookup("replicas"))
	viper.BindPFlag("pod-ip", pf.Lookup("pod-ip"))

	argToEnv := map[string]string{
		"jobs-dir":                "JOBS_DIR",
		"output-dir":              "OUTPUT_DIR",
		"docker-image-repository": "DOCKER_IMAGE_REPOSITORY",
		"az-index":                "AZ_INDEX",
		"pod-ordinal":             "POD_ORDINAL",
		"replicas":                "REPLICAS",
		"pod-ip":                  bpmconverter.PodIPEnvVar,
	}

	boshManifestFlagCobraSet(pf, argToEnv)
	instanceGroupFlagCobraSet(pf, argToEnv)
	initialRolloutFlagCobraSet(pf, argToEnv)
	cmd.AddEnvToUsage(templateRenderCmd, argToEnv)
}
