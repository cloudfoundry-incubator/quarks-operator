package cmd

import (
	"encoding/base64"
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
	fmtJSON   OutputFormat = "json"
	fmtYAML   OutputFormat = "yaml"
	fmtEncode OutputFormat = "encode"
)

// Set validates and sets the value of the OutputFormat
func (f *OutputFormat) Set(s string) error {
	for _, of := range []OutputFormat{fmtJSON, fmtYAML, fmtEncode} {
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
	variableInterpolationCmd.Flags().VarP(&i.OutputFormat, "format", "f", "output manifest in specified format, one of: json|yaml|encode (default yaml)")
	variableInterpolationCmd.Flags().String("encode-key", "interpolated-manifest", "output key in encode format")

	// This will get the values from any set ENV var, but always
	// the values provided via the flags have more precedence.
	viper.AutomaticEnv()

	viper.BindPFlag("manifest", variableInterpolationCmd.Flags().Lookup("manifest"))
	viper.BindPFlag("variables_dir", variableInterpolationCmd.Flags().Lookup("variables-dir"))
	viper.BindPFlag("encode-key", variableInterpolationCmd.Flags().Lookup("encode-key"))
}

func (i *initCmd) runVariableInterpolationCmd(cmd *cobra.Command, args []string) error {
	if i.OutputFormat == "" {
		i.OutputFormat = fmtYAML
	}

	defer log.Sync()

	var vars []boshtpl.Variables

	manifestFile := viper.GetString("manifest")
	variablesDir := filepath.Clean(viper.GetString("variables_dir"))
	encodeKey := viper.GetString("encode-key")

	if _, err := os.Stat(manifestFile); os.IsNotExist(err) {
		return errors.Errorf("no such variable: %s", manifestFile)
	}

	info, err := os.Stat(variablesDir)
	if os.IsNotExist(err) || !info.IsDir() {
		return errors.Errorf("no such directory: %s", variablesDir)
	}

	// Read files
	mBytes, err := ioutil.ReadFile(manifestFile)
	if err != nil {
		return errors.Wrapf(err, "could not read manifest variable")
	}

	variables, err := ioutil.ReadDir(variablesDir)
	if err != nil {
		return errors.Wrapf(err, "could not read variables directory")
	}

	for _, variable := range variables {
		// Each directory is a variable name
		if variable.IsDir() {
			staticVars := boshtpl.StaticVariables{}
			// Each filename is a field name and its context is a variable value
			err = filepath.Walk(filepath.Clean(variablesDir+"/"+variable.Name()), func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !info.IsDir() {
					_, varFileName := filepath.Split(path)
					// Skip the symlink to a directory
					if strings.HasPrefix(varFileName, "..") {
						return filepath.SkipDir
					}
					varBytes, err := ioutil.ReadFile(path)
					if err != nil {
						log.Fatal(errors.Wrapf(err, "could not read variables variable"))
					}

					// Find variable type is password, set password value directly
					if varFileName == "password" {
						staticVars[variable.Name()] = string(varBytes)
					} else {
						staticVars[variable.Name()] = mergeStaticVar(staticVars[variable.Name()], varFileName, string(varBytes))
					}
				}
				return nil
			})
			if err != nil {
				return errors.Wrapf(err, "could not read directory  %s", variable.Name())
			}

			// Re-unmarshal staticVars
			bytes, err := yaml.Marshal(staticVars)
			if err != nil {
				return errors.Wrapf(err, "could not marshal variables: %s", string(bytes))
			}

			err = yaml.Unmarshal(bytes, &staticVars)
			if err != nil {
				return errors.Wrapf(err, "could not unmarshal variables: %s", string(bytes))
			}

			vars = append(vars, staticVars)
		}
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
	case string(fmtJSON):
		jsonBytes, err := yamlUtil.ToJSON(bytes)
		if err != nil {
			return err
		}
		fmt.Println(string(jsonBytes))
	case string(fmtEncode):
		sEnc := base64.StdEncoding.EncodeToString(bytes)
		fmt.Println(fmt.Sprintf(`{"%s":"%s"}`, encodeKey, sEnc))
	case string(fmtYAML):
		fmt.Println("---")
		fmt.Println(string(bytes))
	default:
		return fmt.Errorf("unknown output format: %q", i.OutputFormat)
	}

	return nil
}

func mergeStaticVar(staticVar interface{}, field string, value string) interface{} {
	if staticVar == nil {
		staticVar = map[string]interface{}{
			field: value,
		}
	} else {
		staticVarMap := staticVar.(map[string]interface{})
		staticVarMap[field] = value
		staticVar = staticVarMap
	}

	return staticVar
}
