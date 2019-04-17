package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

// dataGatherCmd represents the dataGather command
var dataGatherCmd = &cobra.Command{
	Use:   "data-gather [flags]",
	Short: "Gathers data of a bosh manifest",
	Long: `Gathers data of a manifest. 

This will retrieve information of an instance-group
inside a bosh manifest file.

`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log = newLogger()
		defer log.Sync()
		boshManifestPath := viper.GetString("bosh-manifest-path")
		if len(boshManifestPath) == 0 {
			return fmt.Errorf("manifest cannot be empty")
		}

		baseDir := viper.GetString("base-dir")
		if len(baseDir) == 0 {
			return fmt.Errorf("base directory cannot be empty")
		}

		cfOperatorNamespace := viper.GetString("cf-operator-namespace")
		if len(cfOperatorNamespace) == 0 {
			return fmt.Errorf("namespace cannot be empty")
		}

		instanceGroupName := viper.GetString("instance-group-name")

		boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
		if err != nil {
			return err
		}

		boshManifestStruct := manifest.Manifest{}
		err = yaml.Unmarshal(boshManifestBytes, &boshManifestStruct)
		if err != nil {
			return err
		}

		dg := manifest.NewDataGatherer(log, &boshManifestStruct)
		result, err := dg.GenerateManifest(baseDir, cfOperatorNamespace, instanceGroupName)
		if err != nil {
			return err
		}

		jsonBytes, err := json.Marshal(map[string]string{
			"properties.yaml": string(result),
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
	},
}

func init() {
	rootCmd.AddCommand(dataGatherCmd)

	dataGatherCmd.Flags().StringP("bosh-manifest-path", "m", "", "path to a bosh manifest file")
	dataGatherCmd.Flags().String("kubernetes-namespace", "", "the kubernetes namespace")
	dataGatherCmd.Flags().StringP("base-dir", "b", "", "a path to the base directory")
	dataGatherCmd.Flags().StringP("instance-group-name", "g", "", "name of the instance group for data gathering")

	viper.BindPFlag("bosh-manifest-path", dataGatherCmd.Flags().Lookup("bosh-manifest-path"))
	viper.BindPFlag("kubernetes-namespace", dataGatherCmd.Flags().Lookup("kubernetes-namespace"))
	viper.BindPFlag("base-dir", dataGatherCmd.Flags().Lookup("base-dir"))
	viper.BindPFlag("instance-group-name", dataGatherCmd.Flags().Lookup("instance-group-name"))

	argToEnv["bosh-manifest-path"] = "BOSH_MANIFEST_PATH"
	argToEnv["instance-group-name"] = "INSTANCE_GROUP_NAME"
	argToEnv["kubernetes-namespace"] = "KUBERNETES_NAMESPACE"
	argToEnv["base-dir"] = "BASE_DIR"

	for arg, env := range argToEnv {
		viper.BindEnv(arg, env)
	}
}
