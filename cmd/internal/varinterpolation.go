package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// variableInterpolationCmd represents the variableInterpolation command
var variableInterpolationCmd = &cobra.Command{
	Use:   "variable-interpolation [flags]",
	Short: "Interpolate variables",
	Long: `Interpolate variables of a manifest:

This will interpolate all the variables found in a 
manifest into kubernetes resources.

`,
	Run: func(cmd *cobra.Command, args []string) {
		// All flag values should be reachable via the viper.GetString("viper_flag")
	},
}

func init() {
	rootCmd.AddCommand(variableInterpolationCmd)
	variableInterpolationCmd.Flags().StringP("manifest", "m", "", "path to a bosh manifest")
	variableInterpolationCmd.Flags().StringP("variables-dir", "v", "", "path to the variables dir")

	// This will get the values from any set ENV var, but always
	// the values provided via the flags have more precedence.
	viper.AutomaticEnv()

	viper.BindPFlag("manifest", variableInterpolationCmd.Flags().Lookup("manifest"))
	viper.BindPFlag("variables_dir", variableInterpolationCmd.Flags().Lookup("variables-dir"))
}
