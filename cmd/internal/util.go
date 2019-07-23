package cmd

import (
	"github.com/spf13/cobra"
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

	utilCmd.PersistentFlags().StringP("bosh-manifest-path", "m", "", "path to the bosh manifest file")
	utilCmd.PersistentFlags().StringP("instance-group-name", "g", "", "name of the instance group for data gathering")
	utilCmd.PersistentFlags().StringP("base-dir", "b", "", "a path to the base directory")
	utilCmd.PersistentFlags().StringP("logs-dir", "z", "", "a path from where to tail logs")

	viper.BindPFlag("bosh-manifest-path", utilCmd.PersistentFlags().Lookup("bosh-manifest-path"))
	viper.BindPFlag("instance-group-name", utilCmd.PersistentFlags().Lookup("instance-group-name"))
	viper.BindPFlag("base-dir", utilCmd.PersistentFlags().Lookup("base-dir"))
	viper.BindPFlag("logs-dir", utilCmd.PersistentFlags().Lookup("logs-dir"))

	argToEnv := map[string]string{
		"base-dir":            "BASE_DIR",
		"bosh-manifest-path":  "BOSH_MANIFEST_PATH",
		"instance-group-name": "INSTANCE_GROUP_NAME",
		"logs-dir":            "LOGS_DIR",
	}
	AddEnvToUsage(utilCmd, argToEnv)

}
