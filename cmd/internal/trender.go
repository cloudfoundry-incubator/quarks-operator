package cmd

import (
	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

// templateRenderCmd represents the dataGather command
var templateRenderCmd = &cobra.Command{
	Use:   "template-render [flags]",
	Short: "Renders a bosh manifest",
	Long: `Renders a bosh manifest.

This will render a provided manifest instance-group
and will generate the output into the specified output
directory.

`,
	Run: func(cmd *cobra.Command, args []string) {
		// Note: for retrieving the values of the job-dir flag, use viper.GetStringSlice("job_dir"))
		// All other flag values should be reachable via the viper.GetString("viper_flag")
	},
}

func init() {
	rootCmd.AddCommand(templateRenderCmd)
	templateRenderCmd.Flags().StringSliceP("job-dir", "j", []string{}, "path to the job dirs. The flag can be specify multiple times")
	templateRenderCmd.Flags().StringP("instance-group-manifest", "i", "", "location of the ig manifest")
	templateRenderCmd.Flags().StringP("output-dir", "d", "", "path to a directory to store the output")

	// This will get the values from any set ENV var, but always
	// the values provided via the flags have more precedence.
	viper.AutomaticEnv()
	viper.BindPFlag("job_dir", templateRenderCmd.Flags().Lookup("job-dir"))
	viper.BindPFlag("instance_group_manifest", templateRenderCmd.Flags().Lookup("instance-group-manifest"))
	viper.BindPFlag("output_dir", templateRenderCmd.Flags().Lookup("output-dir"))

}
