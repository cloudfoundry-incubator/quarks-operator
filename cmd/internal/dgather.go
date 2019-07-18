package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

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
	RunE: func(cmd *cobra.Command, args []string) (err error) {
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

		resolvedProperties, err := dg.ResolvedProperties()
		if err != nil {
			return err
		}

		propertiesBytes, err := yaml.Marshal(resolvedProperties)
		if err != nil {
			return err
		}

		jsonBytes, err := json.Marshal(map[string]string{
			"properties.yaml": string(propertiesBytes),
		})
		if err != nil {
			return errors.Wrapf(err, "could not marshal json output")
		}

		// Close w, and restore the original stdOut
		w.Close()
		os.Stdout = origStdOut

		var buf bytes.Buffer
		io.Copy(&buf, r)

		if buf.Len() > 0 {
			return errors.Errorf("unexpected data sent to stdOut, during the data-gather cmd: %s", buf.String())
		}
		// Write to an original stdOut
		// without any undesired data
		f := bufio.NewWriter(os.Stdout)

		// Ensure bufio.NewWriter will send
		// data at the very end of this func
		// Therefore, we can tail=1 in other
		// containers, and we will get the
		// correct data.
		defer f.Flush()
		_, err = f.Write(jsonBytes)
		if err != nil {
			return err
		}

		return nil
	},
}

func init() {
	utilCmd.AddCommand(dataGatherCmd)
}
