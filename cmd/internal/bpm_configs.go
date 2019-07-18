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

// bpmConfigsCmd calculates and prints the BPM configs for all BOSH jobs of a given instance group
var bpmConfigsCmd = &cobra.Command{
	Use:   "bpm-configs [flags]",
	Short: "Prints the BPM configs for all BOSH jobs of an instance group",
	Long: `Prints the BPM configs for all BOSH jobs of an instance group.

This command calculates and prints the BPM configurations for all all BOSH jobs of a given
instance group.
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

		// Close w, and restore the original stdOut
		w.Close()
		os.Stdout = origStdOut

		var buf bytes.Buffer
		io.Copy(&buf, r)

		if buf.Len() > 0 {
			return errors.Errorf("unexpected data sent to stdout, during the data bpm-configs cmd: %s", buf.String())
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
	utilCmd.AddCommand(bpmConfigsCmd)
}
