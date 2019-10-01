package cmd

import (
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// persistOutputCmd is the persist-output command.
var persistOutputCmd = &cobra.Command{
	Use:   "persist-output [flags]",
	Short: "Persist a file into a kube secret",
	Long: `Persists a log file created by containers in a pod of extendedjob
	
into a versionsed secret or kube native secret using flags specified to this command.
`,
	RunE: func(cmd *cobra.Command, args []string) (err error) {

		namespace := viper.GetString("cf-operator-namespace")
		if len(namespace) == 0 {
			return errors.Errorf("persist-output command failed. cf-operator-namespace flag is empty.")
		}

		return extendedjob.PersistOutput(namespace)
	},
}

func init() {
	utilCmd.AddCommand(persistOutputCmd)
}
