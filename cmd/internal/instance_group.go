package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	quarksutils "code.cloudfoundry.org/quarks-utils/pkg/cmd"
)

const dGatherFailedMessage = "instance-group command failed."

// instanceGroupCmd command to create an instance group manifest where all
// properties are resolved
var instanceGroupCmd = &cobra.Command{
	Use:   "instance-group [flags]",
	Short: "Resolves instance group properties of a BOSH manifest",
	Long: `Resolves instance group properties of a BOSH manifest.

This will resolve the properties of an instance group and return a manifest for that instance group.

`,
	PreRun: func(cmd *cobra.Command, args []string) {
		quarksutils.BOSHManifestFlagViperBind(cmd.Flags())
		quarksutils.BaseDirFlagViperBind(cmd.Flags())
		quarksutils.InstanceGroupFlagViperBind(cmd.Flags())
		quarksutils.OutputFilePathFlagViperBind(cmd.Flags())
	},

	RunE: func(_ *cobra.Command, args []string) (err error) {
		defer func() {
			if err != nil {
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

		boshManifestPath, err := quarksutils.BOSHManifestFlagValidation(dGatherFailedMessage)
		if err != nil {
			return err
		}

		baseDir, err := quarksutils.BaseDirFlagValidation(dGatherFailedMessage)
		if err != nil {
			return err
		}

		namespace := viper.GetString("cf-operator-namespace")
		if len(namespace) == 0 {
			return errors.Errorf("%s cf-operator-namespace flag is empty.", dGatherFailedMessage)
		}

		outputFilePath, err := quarksutils.OutputFilePathFlagValidation(dGatherFailedMessage)
		if err != nil {
			return err
		}

		instanceGroupName, err := quarksutils.InstanceGroupFlagValidation(dGatherFailedMessage)
		if err != nil {
			return err
		}

		boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
		if err != nil {
			return errors.Wrapf(err, "%s Reading file specified in the bosh-manifest-path flag failed. Please check the filepath to continue.", dGatherFailedMessage)
		}

		m, err := manifest.LoadYAML(boshManifestBytes)
		if err != nil {
			return errors.Wrapf(err, "%s Loading bosh manifest file failed. Please check the file contents and try again.", dGatherFailedMessage)
		}

		dg, err := manifest.NewInstanceGroupResolver(baseDir, namespace, *m, instanceGroupName)
		if err != nil {
			return errors.Wrapf(err, dGatherFailedMessage)
		}

		manifest, err := dg.Manifest()
		if err != nil {
			return errors.Wrapf(err, dGatherFailedMessage)
		}

		propertiesBytes, err := manifest.Marshal()
		if err != nil {
			return errors.Wrapf(err, "%s YAML marshalling instance group manifest failed.", dGatherFailedMessage)
		}

		jsonBytes, err := json.Marshal(map[string]string{
			"properties.yaml": string(propertiesBytes),
		})
		if err != nil {
			return errors.Wrapf(err, "%s JSON marshalling instance group manifest failed.", dGatherFailedMessage)
		}

		// Close w, and restore the original stdOut
		w.Close()
		os.Stdout = origStdOut

		var buf bytes.Buffer
		io.Copy(&buf, r)

		if buf.Len() > 0 {
			return errors.Errorf("unexpected data sent to stdOut, during the instance-group cmd: %s", buf.String())
		}
		err = ioutil.WriteFile(outputFilePath, jsonBytes, 0644)
		if err != nil {
			return errors.Wrapf(err, "%s Writing json into a output file failed.", dGatherFailedMessage)
		}

		return nil
	},
}

func init() {
	utilCmd.AddCommand(instanceGroupCmd)

	pf := instanceGroupCmd.PersistentFlags()
	argToEnv := map[string]string{}

	quarksutils.BOSHManifestFlagCobraSet(pf, argToEnv)
	quarksutils.BaseDirFlagCobraSet(pf, argToEnv)
	quarksutils.InstanceGroupFlagCobraSet(pf, argToEnv)
	quarksutils.OutputFilePathFlagCobraSet(pf, argToEnv)
	quarksutils.AddEnvToUsage(instanceGroupCmd, argToEnv)
}
