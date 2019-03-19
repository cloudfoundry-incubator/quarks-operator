package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	boshtpl "github.com/cloudfoundry/bosh-cli/director/template"
	"github.com/cppforlife/go-patch/patch"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

// Variables represents a map of BOSH variables
type Variables map[string]interface{}

// variableInterpolationCmd represents the variableInterpolation command
var variableInterpolationCmd = &cobra.Command{
	Use:   "variable-interpolation [flags]",
	Short: "Interpolate variables",
	Long: `Interpolate variables of a manifest:

This will interpolate all the variables found in a 
manifest into kubernetes resources.

`,
	RunE: runVariableInterpolationCmd,
}

func init() {
	rootCmd.AddCommand(variableInterpolationCmd)
	variableInterpolationCmd.Flags().StringP("manifest", "m", "", "path to a bosh manifest")
	variableInterpolationCmd.Flags().StringP("variables-dir", "v", "", "path to the variables dir")

	// This will get the values from any set ENV var, but always
	// the values provided via the flags have more precedence.
	viper.AutomaticEnv()

	viper.BindPFlag("manifest", variableInterpolationCmd.Flags().Lookup("manifest"))
	viper.BindPFlag("variables_dir", variableInterpolationCmd.Flags().Lookup("variables-dir"))
}

func runVariableInterpolationCmd(cmd *cobra.Command, args []string) error {
	defer log.Sync()

	var vars []boshtpl.Variables

	manifestFile := viper.GetString("manifest")
	variablesDir := viper.GetString("variables_dir")

	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		return errors.Errorf("no such file: %s", manifestFile)
	}

	info, err := os.Stat(variablesDir)
	if os.IsNotExist(err) || !info.IsDir() {
		return errors.Errorf("no such directory: %s", variablesDir)
	}

	// Read files
	log.Debugf("Reading manifest file: %s", manifestFile)
	mBytes, err := ioutil.ReadFile(manifestFile)
	if err != nil {
		return errors.Wrapf(err, "could not read manifest file")
	}

	err = filepath.Walk(variablesDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			extension := filepath.Ext(strings.TrimSpace(path))
			if extension == ".yml" || extension == ".yaml" {
				log.Debugf("Reading variables file: %s", path)
				varBytes, err := ioutil.ReadFile(path)
				if err != nil {
					log.Fatal(errors.Wrapf(err, "could not read variables file"))
				}
				staticVars := boshtpl.StaticVariables{}

				err = yaml.Unmarshal(varBytes, &staticVars)
				if err != nil {
					return errors.Wrapf(err, "could not deserialize variables: %s", string(varBytes))
				}

				vars = append(vars, staticVars)
			}
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(err, "could not read variables directory")
	}

	multiVars := boshtpl.NewMultiVars(vars)
	tpl := boshtpl.NewTemplate(mBytes)

	// Following options are empty for cf-operator
	op := patch.Ops{}
	evalOpts := boshtpl.EvaluateOpts{
		ExpectAllKeys:     false,
		ExpectAllVarsUsed: false,
	}

	bytes, err := tpl.Evaluate(multiVars, op, evalOpts)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate variables")
	}

	fmt.Println("---\n# Manifest:")
	fmt.Println(string(bytes))
	return nil
}
