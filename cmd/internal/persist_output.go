package cmd

import (
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"code.cloudfoundry.org/quarks-job/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/quarks-job/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/quarks-utils/pkg/cmd"
	kubeConfig "code.cloudfoundry.org/quarks-utils/pkg/kubeconfig"
)

// TODO this will be removed once master is passing tests
const persistNamespace = "operator-namespace"

// persistOutputCmd is the persist-output command.
var persistOutputCmd = &cobra.Command{
	Use:   "persist-output [flags]",
	Short: "Persist a file into a kube secret",
	Long: `Persists a log file created by containers in a pod of extendedjob
	
into a versionsed secret or kube native secret using flags specified to this command.
`,
	RunE: func(_ *cobra.Command, args []string) (err error) {

		namespace := viper.GetString(persistNamespace)
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
	rootCmd.AddCommand(persistOutputCmd)
	pf := rootCmd.PersistentFlags()

	pf.StringP(persistNamespace, "", "default", "The operator namespace")
	viper.BindPFlag(persistNamespace, pf.Lookup(persistNamespace))

	argToEnv := map[string]string{persistNamespace: "OPERATOR_NAMESPACE"}
	cmd.AddEnvToUsage(rootCmd, argToEnv)
}

// authenticateInCluster authenticates with the in cluster and returns the client
func authenticateInCluster() (*kubernetes.Clientset, *versioned.Clientset, error) {

	log = cmd.Logger(zap.AddCallerSkip(1))
	defer log.Sync()

	config, err := kubeConfig.NewGetter(log).Get("")
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Couldn't fetch Kubeconfig. Ensure kubeconfig is present to continue.")
	}
	if err := kubeConfig.NewChecker(log).Check(config); err != nil {
		return nil, nil, errors.Wrapf(err, "Couldn't check Kubeconfig. Ensure kubeconfig is correct to continue.")
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
