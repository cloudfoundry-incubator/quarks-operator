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

func boshManifestFlagValidation(cmdMsg string) (string, error) {
	boshManifestPath := viper.GetString("bosh-manifest-path")
	if len(boshManifestPath) == 0 {
		return "", errors.Errorf("%s bosh-manifest-path flag is empty.", cmdMsg)
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

func baseDirFlagValidation(cmdMsg string) (string, error) {
	baseDir := viper.GetString("base-dir")
	if len(baseDir) == 0 {
		return "", errors.Errorf("%s base-dir flag is empty.", cmdMsg)
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

func instanceGroupFlagValidation(cmdMsg string) (string, error) {
	instanceGroupName := viper.GetString("instance-group-name")
	if len(instanceGroupName) == 0 {
		return "", errors.Errorf("%s instance-group-name flag is empty.", cmdMsg)
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

func outputFilePathFlagValidation(cmdMsg string) (string, error) {
	outputFilePath := viper.GetString("output-file-path")
	if len(outputFilePath) == 0 {
		return "", errors.Errorf("%s output-file-path flag is empty.", cmdMsg)
	}
	return outputFilePath, nil
}

func outputFilePathFlagCobraSet(pf *flag.FlagSet, argToEnv map[string]string) {
	pf.StringP("output-file-path", "", "", "Path of the file to which json output is redirected.")
	argToEnv["output-file-path"] = "OUTPUT_FILE_PATH"
}

func outputFilePathFlagViperBind(pf *flag.FlagSet) {
	viper.BindPFlag("output-file-path", pf.Lookup("output-file-path"))
}
