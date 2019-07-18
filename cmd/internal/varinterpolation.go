package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type initCmd struct {
}

// variableInterpolationCmd represents the variableInterpolation command
var variableInterpolationCmd = &cobra.Command{
	Use:   "variable-interpolation [flags]",
	Short: "Interpolate variables",
	Long: `Interpolate variables of a manifest:

This will interpolate all the variables from a folder and write an
interpolated manifest to STDOUT
`,
}

func init() {
	i := &initCmd{}

	variableInterpolationCmd.RunE = i.runVariableInterpolationCmd
	utilCmd.AddCommand(variableInterpolationCmd)
	variableInterpolationCmd.Flags().StringP("variables-dir", "v", "", "path to the variables dir")

	viper.BindPFlag("variables-dir", variableInterpolationCmd.Flags().Lookup("variables-dir"))

	argToEnv := map[string]string{
		"variables-dir": "VARIABLES_DIR",
	}
	AddEnvToUsage(variableInterpolationCmd, argToEnv)

}

func (i *initCmd) runVariableInterpolationCmd(cmd *cobra.Command, args []string) (err error) {
	defer func() {
		if err != nil {
			time.Sleep(debugGracePeriod)
		}
	}()

	log = newLogger()
	defer log.Sync()

	boshManifestPath := viper.GetString("bosh-manifest-path")
	variablesDir := filepath.Clean(viper.GetString("variables-dir"))

	if _, err := os.Stat(boshManifestPath); os.IsNotExist(err) {
		return errors.Errorf("no such file: %s", boshManifestPath)
	}

	info, err := os.Stat(variablesDir)

	if os.IsNotExist(err) {
		return errors.Errorf("directory %s doesn't exist", variablesDir)
	} else if err != nil {
		return errors.Errorf("error on dir stat: %s", variablesDir)
	} else if !info.IsDir() {
		return errors.Errorf("path %s is not a directory", variablesDir)
	}

	// Read files
	boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
	if err != nil {
		return errors.Wrapf(err, "could not read manifest variable")
	}

	return manifest.InterpolateVariables(log, boshManifestBytes, variablesDir)
}
