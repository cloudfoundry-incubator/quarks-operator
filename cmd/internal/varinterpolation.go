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
	yamlUtil "k8s.io/apimachinery/pkg/util/yaml"
)

type initCmd struct {
	OutputFormat OutputFormat
}

// OutputFormat defines valid values for init output (json, yaml)
type OutputFormat string

// String returns the string value of the OutputFormat
func (f *OutputFormat) String() string {
	return string(*f)
}

// Type returns the string value of the OutputFormat
func (f *OutputFormat) Type() string {
	return "OutputFormat"
}

const (
	fmtJSON OutputFormat = "json"
	fmtYAML OutputFormat = "yaml"
)

// Set validates and sets the value of the OutputFormat
func (f *OutputFormat) Set(s string) error {
	for _, of := range []OutputFormat{fmtJSON, fmtYAML} {
		if s == string(of) {
			*f = of
			return nil
		}
	}
	return fmt.Errorf("unknown output format %q", s)
}

// variableInterpolationCmd represents the variableInterpolation command
var variableInterpolationCmd = &cobra.Command{
	Use:   "variable-interpolation [flags]",
	Short: "Interpolate variables",
	Long: `Interpolate variables of a manifest:

This will interpolate all the variables found in a 
manifest into kubernetes resources.

`,
}

func init() {
	i := &initCmd{}

	variableInterpolationCmd.RunE = i.runVariableInterpolationCmd
	rootCmd.AddCommand(variableInterpolationCmd)
	variableInterpolationCmd.Flags().StringP("manifest", "m", "", "path to a bosh manifest")
	variableInterpolationCmd.Flags().StringP("variables-dir", "v", "", "path to the variables dir")
	variableInterpolationCmd.Flags().VarP(&i.OutputFormat, "format", "f", "output manifest in specified format (json or yaml)")

	// This will get the values from any set ENV var, but always
	// the values provided via the flags have more precedence.
	viper.AutomaticEnv()

	viper.BindPFlag("manifest", variableInterpolationCmd.Flags().Lookup("manifest"))
	viper.BindPFlag("variables_dir", variableInterpolationCmd.Flags().Lookup("variables-dir"))
}

func (i *initCmd) runVariableInterpolationCmd(cmd *cobra.Command, args []string) error {
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
	mBytes, err := ioutil.ReadFile(manifestFile)
	if err != nil {
		return errors.Wrapf(err, "could not read manifest file")
	}

	err = filepath.Walk(variablesDir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			extension := filepath.Ext(strings.TrimSpace(path))
			if extension == ".yml" || extension == ".yaml" {
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

	switch i.OutputFormat.String() {
	case "json":
		jsonBytes, err := yamlUtil.ToJSON(bytes)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
	case "yaml":
	case "":
		fmt.Println("---")
		fmt.Println(string(bytes))
	default:
		return fmt.Errorf("unknown output format: %q", i.OutputFormat)
	}

	return nil
}
