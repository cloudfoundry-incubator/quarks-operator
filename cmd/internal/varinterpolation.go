package cmd

import (
	"bufio"
	"encoding/json"
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

type initCmd struct {
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
	variableInterpolationCmd.Flags().StringP("bosh-manifest-path", "m", "", "path to a bosh manifest")
	variableInterpolationCmd.Flags().StringP("variables-dir", "v", "", "path to the variables dir")

	viper.BindPFlag("bosh-manifest-path", variableInterpolationCmd.Flags().Lookup("bosh-manifest-path"))
	viper.BindPFlag("variables-dir", variableInterpolationCmd.Flags().Lookup("variables-dir"))

	argToEnv["bosh-manifest-path"] = "BOSH_MANIFEST_PATH"
	argToEnv["variables-dir"] = "VARIABLES_DIR"

	for arg, env := range argToEnv {
		viper.BindEnv(arg, env)
	}
}

func (i *initCmd) runVariableInterpolationCmd(cmd *cobra.Command, args []string) error {
	defer log.Sync()

	boshManifestPath := viper.GetString("bosh-manifest-path")
	variablesDir := filepath.Clean(viper.GetString("variables-dir"))

	if _, err := os.Stat(boshManifestPath); os.IsNotExist(err) {
		return errors.Errorf("no such variable: %s", boshManifestPath)
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

	variables, err := ioutil.ReadDir(variablesDir)
	if err != nil {
		return errors.Wrapf(err, "could not read variables directory")
	}

	var vars []boshtpl.Variables

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
	tpl := boshtpl.NewTemplate(boshManifestBytes)

	// Following options are empty for cf-operator
	op := patch.Ops{}
	evalOpts := boshtpl.EvaluateOpts{
		ExpectAllKeys:     false,
		ExpectAllVarsUsed: false,
	}

	yamlBytes, err := tpl.Evaluate(multiVars, op, evalOpts)
	if err != nil {
		return errors.Wrapf(err, "could not evaluate variables")
	}

	jsonBytes, err := json.Marshal(map[string]string{
		"manifest.yaml": string(yamlBytes),
	})
	if err != nil {
		return errors.Wrapf(err, "could not marshal json output")
	}

	f := bufio.NewWriter(os.Stdout)
	defer f.Flush()
	_, err = f.Write(jsonBytes)
	if err != nil {
		return err
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
