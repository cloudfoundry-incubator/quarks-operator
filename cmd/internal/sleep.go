package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

// dataGatherCmd represents the dataGather command
var sleepCommand = &cobra.Command{
	Use:   "sleep",
	Short: "Sleep forever",
	Long: `Do nothing forever.

This is used as a dummy command for the volume-management-pods.

`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {
		return Sleep()
	},
}

// Sleep will sleep indefinitely
func Sleep() error {
	for {
		time.Sleep(time.Hour * 24)
	}
	return nil
}

func init() {
	utilCmd.AddCommand(sleepCommand)
}
