package cmd

import (
	"fmt"
	"net"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(waitCmd)
}

var waitCmd = &cobra.Command{
	Use:   "wait",
	Short: "Wait for required service",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		for {
			if _, err := net.LookupIP(args[0]); err == nil {
				break
			}
			fmt.Printf("Waiting for %s to be reachable\n", args[0])
			time.Sleep(time.Second * 5)
		}
	},
}
