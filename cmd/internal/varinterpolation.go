package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
)

const (
	vInterpolateFailedMessage = "variable-interpolation command failed."
)

// variableInterpolationCmd represents the variableInterpolation command
var variableInterpolationCmd = &cobra.Command{
	Use:   "variable-interpolation [flags]",
	Short: "Interpolate variables",
	Long: `Interpolate variables of a manifest:

This will interpolate all the variables from a folder and write an
interpolated manifest to STDOUT
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		boshManifestFlagViperBind(cmd.Flags())
		outputFilePathFlagViperBind(cmd.Flags())
	},
	RunE: func(_ *cobra.Command, args []string) (err error) {
		defer func() {
			if err != nil {
				time.Sleep(debugGracePeriod)
			}
		}()

		log = cmd.Logger()
		defer log.Sync()

		boshManifestPath, err := boshManifestFlagValidation(vInterpolateFailedMessage)
		if err != nil {
			return err
		}
		outputFilePath, err := outputFilePathFlagValidation(vInterpolateFailedMessage)
		if err != nil {
			return err
		}

		variablesDir := filepath.Clean(viper.GetString("variables-dir"))

		if _, err := os.Stat(boshManifestPath); os.IsNotExist(err) {
			return errors.Errorf("%s bosh-manifest-path file doesn't exist : %s", vInterpolateFailedMessage, boshManifestPath)
		}

		info, err := os.Stat(variablesDir)

		if os.IsNotExist(err) {
			return errors.Errorf("%s %s doesn't exist", vInterpolateFailedMessage, variablesDir)
		} else if err != nil {
			return errors.Errorf("%s Error on dir stat %s", vInterpolateFailedMessage, variablesDir)
		} else if !info.IsDir() {
			return errors.Errorf("%s %s is not a directory", vInterpolateFailedMessage, variablesDir)
		}

		// Read files
		boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
		if err != nil {
			return errors.Wrapf(err, "%s Reading file specified in the bosh-manifest-path flag failed", vInterpolateFailedMessage)
		}

		return manifest.InterpolateVariables(log, boshManifestBytes, variablesDir, outputFilePath)
	},
}

func init() {
	utilCmd.AddCommand(variableInterpolationCmd)
	variableInterpolationCmd.Flags().StringP("variables-dir", "v", "", "path to the variables dir")

	viper.BindPFlag("variables-dir", variableInterpolationCmd.Flags().Lookup("variables-dir"))

	argToEnv := map[string]string{
		"variables-dir": "VARIABLES_DIR",
	}

	pf := variableInterpolationCmd.Flags()
	boshManifestFlagCobraSet(pf, argToEnv)
	outputFilePathFlagCobraSet(pf, argToEnv)

	cmd.AddEnvToUsage(variableInterpolationCmd, argToEnv)

}
