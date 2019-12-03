package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// UtilCmd represents the util subcommand
var utilCmd = &cobra.Command{
	Use:   "util",
	Short: "Calls a utility subcommand",
	Long:  `Calls a utility subcommand.`,
}

func init() {
	rootCmd.AddCommand(utilCmd)
}

func boshManifestFlagValidation() (string, error) {
	boshManifestPath := viper.GetString("bosh-manifest-path")
	if len(boshManifestPath) == 0 {
		return "", errors.New("bosh-manifest-path flag is empty")
	}
	return boshManifestPath, nil
}

func boshManifestFlagCobraSet(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.StringP("bosh-manifest-path", "m", "", "path to the bosh manifest file")
	argToEnv["bosh-manifest-path"] = "BOSH_MANIFEST_PATH"
}

func boshManifestFlagViperBind(pf *flag.FlagSet) {
	viper.BindPFlag("bosh-manifest-path", pf.Lookup("bosh-manifest-path"))

}

func baseDirFlagValidation() (string, error) {
	baseDir := viper.GetString("base-dir")
	if len(baseDir) == 0 {
		return "", errors.New("base-dir flag is empty")
	}
	return baseDir, nil
}

func baseDirFlagCobraSet(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.StringP("base-dir", "b", "", "a path to the base directory")
	argToEnv["base-dir"] = "BASE_DIR"
}

func baseDirFlagViperBind(pf *flag.FlagSet) {
	viper.BindPFlag("base-dir", pf.Lookup("base-dir"))
}

func instanceGroupFlagValidation() (string, error) {
	instanceGroupName := viper.GetString("instance-group-name")
	if len(instanceGroupName) == 0 {
		return "", errors.New("instance-group-name flag is empty")
	}
	return instanceGroupName, nil
}

func instanceGroupFlagCobraSet(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.StringP("instance-group-name", "g", "", "name of the instance group for data gathering")
	argToEnv["instance-group-name"] = "INSTANCE_GROUP_NAME"
}

func instanceGroupFlagViperBind(pf *flag.FlagSet) {
	viper.BindPFlag("instance-group-name", pf.Lookup("instance-group-name"))
}

func outputFilePathFlagValidation() (string, error) {
	outputFilePath := viper.GetString("output-file-path")
	if len(outputFilePath) == 0 {
		return "", errors.New("output-file-path flag is empty")
	}
	return outputFilePath, nil
}

func outputFilePathFlagCobraSet(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.StringP("output-file-path", "", "", "Path of the file to which json output is written.")
	argToEnv["output-file-path"] = "OUTPUT_FILE_PATH"
}

func outputFilePathFlagViperBind(pf *flag.FlagSet) {
	viper.BindPFlag("output-file-path", pf.Lookup("output-file-path"))
}

func initialRolloutFlagCobraSet(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.BoolP("initial-rollout", "", true, "Initial rollout of bosh deployment.")
	argToEnv["initial-rollout"] = "INITIAL_ROLLOUT"
}

func initialRolloutFlagViperBind(pf *flag.FlagSet) {
	viper.BindPFlag("initial-rollout", pf.Lookup("initial-rollout"))
}
