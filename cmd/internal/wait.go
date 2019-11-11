package cmd

import (
	"fmt"
	"net"
	"time"

	"github.com/spf13/viper"

	"github.com/spf13/cobra"
)

func init() {
	utilCmd.AddCommand(waitCmd)
	waitCmd.Flags().IntP("timeout", "", 30*60, "timeout in seconds after the required service must be available")
	viper.BindPFlag("timeout", waitCmd.Flags().Lookup("timeout"))
}

// waitCmd is used to wait for a service (e.g. database), which is required for the calling job (e.g. cloud-controller).
// This command is used to implement the update.serial flag in the BOSH manifest
var waitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Wait for required service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		expiry := time.Now().Add(time.Duration(viper.GetInt("timeout")) * time.Second)
		for {
			if time.Now().After(expiry) {
				return fmt.Errorf("timeout during waiting for %s to be reachable", args[0])
			}
			if _, err := net.LookupIP(args[0]); err == nil {
				return nil
			}
			fmt.Printf("Waiting for %s to be reachable\n", args[0])
			time.Sleep(time.Second * 5)
		}
	},
}
