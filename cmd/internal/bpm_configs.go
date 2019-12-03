package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime/debug"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"sigs.k8s.io/yaml"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
)

const bpmFailedMessage = "bpm-configs command failed."

// bpmConfigsCmd calculates and prints the BPM configs for all BOSH jobs of a given instance group
var bpmConfigsCmd = &cobra.Command{
	Use:   "bpm-configs [flags]",
	Short: "Prints the BPM configs for all BOSH jobs of an instance group",
	Long: `Prints the BPM configs for all BOSH jobs of an instance group.

This command calculates and prints the BPM configurations for all all BOSH jobs of a given
instance group.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		boshManifestFlagViperBind(cmd.Flags())
		baseDirFlagViperBind(cmd.Flags())
		instanceGroupFlagViperBind(cmd.Flags())
		outputFilePathFlagViperBind(cmd.Flags())
		initialRolloutFlagViperBind(cmd.Flags())
	},

	RunE: func(_ *cobra.Command, args []string) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("recovered in f: %v\n%s", r, string(debug.Stack()))
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				fmt.Fprintf(os.Stderr, "Sleeping ...\n")
				time.Sleep(debugGracePeriod)
			}
		}()

		log = cmd.Logger()
		defer log.Sync()

		boshManifestPath, err := boshManifestFlagValidation()
		if err != nil {
			return errors.Wrap(err, bpmFailedMessage)
		}

		baseDir, err := baseDirFlagValidation()
		if err != nil {
			return errors.Wrap(err, bpmFailedMessage)
		}

		namespace := viper.GetString("cf-operator-namespace")
		if len(namespace) == 0 {
			return errors.Errorf("%s cf-operator-namespace flag is empty.", bpmFailedMessage)
		}

		outputFilePath, err := outputFilePathFlagValidation()
		if err != nil {
			return errors.Wrap(err, bpmFailedMessage)
		}

		instanceGroupName, err := instanceGroupFlagValidation()
		if err != nil {
			return errors.Wrap(err, bpmFailedMessage)
		}

		boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
		if err != nil {
			return errors.Wrapf(err, "%s Reading file specified in the bosh-manifest-path flag failed.", bpmFailedMessage)
		}

		m, err := manifest.LoadYAML(boshManifestBytes)
		if err != nil {
			return errors.Wrapf(err, "%s Loading bosh manifest file failed. Please check the file contents and try again.", bpmFailedMessage)
		}

		igr, err := manifest.NewInstanceGroupResolver(afero.NewOsFs(), baseDir, *m, instanceGroupName)
		if err != nil {
			return errors.Wrap(err, bpmFailedMessage)
		}

		initialRollout := viper.GetBool("initial-rollout")
		bpmInfo, err := igr.BPMInfo(initialRollout)
		if err != nil {
			return errors.Wrap(err, bpmFailedMessage)
		}

		bpmBytes, err := yaml.Marshal(bpmInfo)
		if err != nil {
			return errors.Wrapf(err, "%s YAML marshalling bpmConfigs spec returned by igr.BPMConfigs() failed.", bpmFailedMessage)
		}

		jsonBytes, err := json.Marshal(map[string]string{
			"bpm.yaml": string(bpmBytes),
		})
		if err != nil {
			return errors.Wrapf(err, "%s JSON marshalling bpmConfigs spec returned by igr.BPMConfigs() failed.", bpmFailedMessage)
		}

		err = ioutil.WriteFile(outputFilePath, jsonBytes, 0644)
		if err != nil {
			return errors.Wrapf(err, "%s Writing bpmConfigs json into a file failed.", bpmFailedMessage)
		}
		return nil
	},
}

func init() {
	utilCmd.AddCommand(bpmConfigsCmd)
	pf := bpmConfigsCmd.Flags()

	argToEnv := map[string]string{}

	boshManifestFlagCobraSet(pf, argToEnv)
	baseDirFlagCobraSet(pf, argToEnv)
	instanceGroupFlagCobraSet(pf, argToEnv)
	outputFilePathFlagCobraSet(pf, argToEnv)
	initialRolloutFlagCobraSet(pf, argToEnv)
	cmd.AddEnvToUsage(bpmConfigsCmd, argToEnv)
}
