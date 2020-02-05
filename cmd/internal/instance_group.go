package cmd

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"sigs.k8s.io/yaml"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
)

const igFailedMessage = "instance-group command failed."

// instanceGroupCmd command to create an instance group manifest where all
// properties are resolved
var instanceGroupCmd = &cobra.Command{
	Use:   "instance-group [flags]",
	Short: "Resolves instance group properties of a BOSH manifest",
	Long: `Resolves instance group properties of a BOSH manifest.

This will resolve the properties of an instance group and return a manifest for that instance group.
Also calculates and prints the BPM configurations for all BOSH jobs of that instance group.

`,
	PreRun: func(cmd *cobra.Command, args []string) {
		boshManifestFlagViperBind(cmd.Flags())
		baseDirFlagViperBind(cmd.Flags())
		deploymentNameFlagViperBind(cmd.Flags())
		instanceGroupFlagViperBind(cmd.Flags())
		outputFilePathFlagViperBind(cmd.Flags())
		initialRolloutFlagViperBind(cmd.Flags())
	},

	RunE: func(_ *cobra.Command, args []string) (err error) {
		defer func() {
			if err != nil {
				time.Sleep(debugGracePeriod)
			}
		}()

		log = cmd.Logger()
		defer log.Sync()

		boshManifestPath, err := boshManifestFlagValidation()
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		baseDir, err := baseDirFlagValidation()
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		namespace := viper.GetString("cf-operator-namespace")
		if len(namespace) == 0 {
			return errors.Errorf("%s cf-operator-namespace flag is empty.", igFailedMessage)
		}

		outputFilePath, err := outputFilePathFlagValidation()
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		deploymentName, err := deploymentNameFlagValidation()
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		instanceGroupName, err := instanceGroupFlagValidation()
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		boshManifestBytes, err := ioutil.ReadFile(boshManifestPath)
		if err != nil {
			return errors.Wrapf(err, "%s Reading file specified in the bosh-manifest-path flag failed. Please check the filepath to continue.", igFailedMessage)
		}

		m, err := manifest.LoadYAML(boshManifestBytes)
		if err != nil {
			return errors.Wrapf(err, "%s Loading BOSH manifest file failed. Please check the file contents and try again.", igFailedMessage)
		}

		dns, err := boshdns.NewDNS(deploymentName, *m)
		if err != nil {
			return errors.Wrapf(err, "%s Loading DNS for BOSH manifest failed.", igFailedMessage)
		}

		igr, err := manifest.NewInstanceGroupResolver(afero.NewOsFs(), baseDir, deploymentName, *m, instanceGroupName, dns)
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		err = igr.CollectQuarksLinks(filepath.Dir(converter.VolumeLinksPath))
		if err != nil {
			return errors.Wrapf(err, "%s failed to collect quarks links.", igFailedMessage)
		}

		initialRollout := viper.GetBool("initial-rollout")
		err = igr.Resolve(initialRollout)
		if err != nil {
			return errors.Wrapf(err, "%s failed to resolve manifest.", igFailedMessage)
		}

		manifest, err := igr.Manifest()
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		err = igr.SaveLinks(outputFilePath)
		if err != nil {
			return errors.Wrapf(err, "%s failed to write link output to file.", igFailedMessage)
		}

		// write instance group manifest
		propertiesBytes, err := manifest.Marshal()
		if err != nil {
			return errors.Wrapf(err, "%s YAML marshalling instance group manifest failed.", igFailedMessage)
		}

		jsonBytes, err := json.Marshal(map[string]string{
			"properties.yaml": string(propertiesBytes),
		})
		if err != nil {
			return errors.Wrapf(err, "%s JSON marshalling instance group manifest failed.", igFailedMessage)
		}

		err = ioutil.WriteFile(filepath.Join(outputFilePath, converter.InstanceGroupOutputFilename), jsonBytes, 0644)
		if err != nil {
			return errors.Wrapf(err, "%s Writing json into a output file failed.", igFailedMessage)
		}

		// write bpm manifest
		bpmInfo, err := igr.BPMInfo()
		if err != nil {
			return errors.Wrap(err, igFailedMessage)
		}

		bpmBytes, err := yaml.Marshal(bpmInfo)
		if err != nil {
			return errors.Wrapf(err, "%s YAML marshalling BPM config spec failed.", igFailedMessage)
		}

		jsonBytes, err = json.Marshal(map[string]string{
			"bpm.yaml": string(bpmBytes),
		})
		if err != nil {
			return errors.Wrapf(err, "%s JSON marshalling BPM config spec failed.", igFailedMessage)
		}

		err = ioutil.WriteFile(filepath.Join(outputFilePath, converter.BPMOutputFilename), jsonBytes, 0644)
		if err != nil {
			return errors.Wrapf(err, "%s Writing BPM config json into a file failed.", igFailedMessage)
		}

		return nil
	},
}

func init() {
	utilCmd.AddCommand(instanceGroupCmd)

	pf := instanceGroupCmd.PersistentFlags()
	argToEnv := map[string]string{}

	boshManifestFlagCobraSet(pf, argToEnv)
	baseDirFlagCobraSet(pf, argToEnv)
	deploymentNameFlagCobraSet(pf, argToEnv)
	instanceGroupFlagCobraSet(pf, argToEnv)
	outputFilePathFlagCobraSet(pf, argToEnv)
	initialRolloutFlagCobraSet(pf, argToEnv)
	cmd.AddEnvToUsage(instanceGroupCmd, argToEnv)
}
