package cmd

import (
	"github.com/spf13/viper"
	"github.com/spf13/cobra"
)

// UtilCmd represents the util subcommand
var utilCmd = &cobra.Command{
	Use: "util",
	Short: "Calls a utility subcommand",
	Long: `Calls a utility subcommand.`,
}

func init() {
	rootCmd.AddCommand(utilCmd)

	utilCmd.PersistentFlags().StringP("bosh-manifest-path", "m", "", "path to the bosh manifest file")
	utilCmd.PersistentFlags().StringP("instance-group-name", "g", "", "name of the instance group for data gathering")

	viper.BindPFlag("bosh-manifest-path", utilCmd.PersistentFlags().Lookup("bosh-manifest-path"))
	viper.BindPFlag("instance-group-name", utilCmd.PersistentFlags().Lookup("instance-group-name"))

	argToEnv := map[string]string{
		"bosh-manifest-path": "BOSH_MANIFEST_PATH",
		"instance-group-name": "INSTANCE_GROUP_NAME",
	}
	AddEnvToUsage(utilCmd, argToEnv)
}