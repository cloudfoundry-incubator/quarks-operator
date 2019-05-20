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

// bpmConfigsCmd calculates and prints the BPM configs for all BOSH jobs of a given instance group
var bpmConfigsCmd = &cobra.Command{
	Use:   "bpm-configs [flags]",
	Short: "Prints the BPM configs for all BOSH jobs of an instance group",
	Long: `Prints the BPM configs for all BOSH jobs of an instance group.

This command calculates and prints the BPM configurations for all all BOSH jobs of a given
instance group.
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

		namespace := viper.GetString("cf-operator-namespace")
		if len(namespace) == 0 {
			return fmt.Errorf("namespace cannot be empty")
		}

		instanceGroupName := viper.GetString("instance-group-name")
		if len(instanceGroupName) == 0 {
			return fmt.Errorf("instance-group-name cannot be empty")
		}

		boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
		if err != nil {
			return err
		}

		m, err := manifest.LoadYAML(boshManifestBytes)
		if err != nil {
			return err
		}

		dg, err := manifest.NewDataGatherer(log, baseDir, namespace, *m, instanceGroupName)
		if err != nil {
			return err
		}

		bpmConfigs, err := dg.BPMConfigs()
		if err != nil {
			return err
		}

		bpmBytes, err := yaml.Marshal(bpmConfigs)
		if err != nil {
			return errors.Wrapf(err, "could not marshal json output")
		}

		jsonBytes, err := json.Marshal(map[string]string{
			"bpm.yaml": string(bpmBytes),
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
	utilCmd.AddCommand(bpmConfigsCmd)
}
