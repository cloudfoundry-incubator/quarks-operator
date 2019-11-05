package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime/debug"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	quarksutils "code.cloudfoundry.org/quarks-utils/pkg/cmd"
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
		quarksutils.BOSHManifestFlagViperBind(cmd.Flags())
		quarksutils.BaseDirFlagViperBind(cmd.Flags())
		quarksutils.InstanceGroupFlagViperBind(cmd.Flags())
		quarksutils.OutputFilePathFlagViperBind(cmd.Flags())
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

		// Store original stdout i
		origStdOut := os.Stdout

		// Dump everything before the JSON bytes buffer creation
		// into w, while we do not want any sort of noise coming
		// into stdout, beside the JSON bytes
		r, w, _ := os.Pipe()
		os.Stdout = w

		log = cmd.Logger()
		defer log.Sync()

		boshManifestPath, err := quarksutils.BOSHManifestFlagValidation(bpmFailedMessage)
		if err != nil {
			return err
		}

		baseDir, err := quarksutils.BaseDirFlagValidation(bpmFailedMessage)
		if err != nil {
			return err
		}

		namespace := viper.GetString("cf-operator-namespace")
		if len(namespace) == 0 {
			return errors.Errorf("%s cf-operator-namespace flag is empty.", bpmFailedMessage)
		}

		outputFilePath, err := quarksutils.OutputFilePathFlagValidation(bpmFailedMessage)
		if err != nil {
			return err
		}

		instanceGroupName, err := quarksutils.InstanceGroupFlagValidation(bpmFailedMessage)
		if err != nil {
			return err
		}

		boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
		if err != nil {
			return errors.Wrapf(err, "%s Reading file specified in the bosh-manifest-path flag failed.", bpmFailedMessage)
		}

		m, err := manifest.LoadYAML(boshManifestBytes)
		if err != nil {
			return errors.Wrapf(err, "%s Loading bosh manifest file failed. Please check the file contents and try again.", bpmFailedMessage)
		}

		dg, err := manifest.NewInstanceGroupResolver(baseDir, namespace, *m, instanceGroupName)
		if err != nil {
			return errors.Wrapf(err, bpmFailedMessage)
		}

		bpmConfigs, err := dg.BPMConfigs()
		if err != nil {
			return errors.Wrapf(err, bpmFailedMessage)
		}

		bpmBytes, err := yaml.Marshal(bpmConfigs)
		if err != nil {
			return errors.Wrapf(err, "%s YAML marshalling bpmConfigs spec returned by dg.BPMConfigs() failed.", bpmFailedMessage)
		}

		jsonBytes, err := json.Marshal(map[string]string{
			"bpm.yaml": string(bpmBytes),
		})
		if err != nil {
			return errors.Wrapf(err, "%s JSON marshalling bpmConfigs spec returned by dg.BPMConfigs() failed.", bpmFailedMessage)
		}

		// Close w, and restore the original stdOut
		w.Close()
		os.Stdout = origStdOut

		var buf bytes.Buffer
		io.Copy(&buf, r)

		if buf.Len() > 0 {
			return errors.Errorf("unexpected data sent to stdout, during the data bpm-configs cmd: %s", buf.String())
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

	quarksutils.BOSHManifestFlagCobraSet(pf, argToEnv)
	quarksutils.BaseDirFlagCobraSet(pf, argToEnv)
	quarksutils.InstanceGroupFlagCobraSet(pf, argToEnv)
	quarksutils.OutputFilePathFlagCobraSet(pf, argToEnv)
	quarksutils.AddEnvToUsage(bpmConfigsCmd, argToEnv)
}
