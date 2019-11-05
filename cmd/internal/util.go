package cmd

import (
	"github.com/spf13/cobra"
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
