package cmd

import (
	"os"

	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

		// hostname of the container is the pod name in kubernetes
		podName, err := os.Hostname()
		if err != nil {
			return errors.Wrapf(err, "failed to fetch pod name.")
		}
		if podName == "" {
			return errors.Wrapf(err, "pod name is empty.")
		}

		// Authenticate with the cluster
		clientSet, versionedClientSet, err := authenticateInCluster()
		if err != nil {
			return err
		}

		po := extendedjob.NewPersistOutputInterface(namespace, podName, clientSet, versionedClientSet, "/mnt/quarks")

		return po.PersistOutput()
	},
}

func init() {
	utilCmd.AddCommand(persistOutputCmd)
}

// authenticateInCluster authenticates with the in cluster and returns the client
func authenticateInCluster() (*kubernetes.Clientset, *versioned.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to authenticate with incluster config")
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create clientset with incluster config")
	}

	versionedClientSet, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to create versioned clientset with incluster config")
	}

	return clientSet, versionedClientSet, nil
}
