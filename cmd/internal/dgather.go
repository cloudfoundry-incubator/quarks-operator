package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// dataGatherCmd represents the dataGather command
var dataGatherCmd = &cobra.Command{
	Use:   "data-gather [flags]",
	Short: "Gathers data of a bosh manifest",
	Long: `Gathers data of a manifest.

This will retrieve information of an instance-group
inside a bosh manifest.

`,
	Run: func(cmd *cobra.Command, args []string) {
		// All flag values should be reachable via the viper.GetString("viper_flag")
	},
}

func init() {
	rootCmd.AddCommand(dataGatherCmd)

	dataGatherCmd.Flags().StringP("manifest", "m", "", "path to a bosh manifest")
	dataGatherCmd.Flags().String("desired-namespace", "", "the kubernetes namespace") //TODO: can we reuse the global ns flag
	dataGatherCmd.Flags().StringP("base-dir", "b", "", "a path to the base directory")
	dataGatherCmd.Flags().StringP("instance-group", "g", "", "the instance-group")

	// This will get the values from any set ENV var, but always
	// the values provided via the flags have more precedence.
	viper.AutomaticEnv()
	viper.BindPFlag("manifest", dataGatherCmd.Flags().Lookup("manifest"))
	viper.BindPFlag("desired_namespace", dataGatherCmd.Flags().Lookup("desired-namespace"))
	viper.BindPFlag("base_dir", dataGatherCmd.Flags().Lookup("base-dir"))
	viper.BindPFlag("instance_group", dataGatherCmd.Flags().Lookup("instance-group"))
}
