package cmd

import (
	"fmt"
	golog "log"
	"os"
	"os/user"
	"path/filepath"

	"code.cloudfoundry.org/cf-operator/pkg/operator"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

var (
	log *zap.SugaredLogger
)

var rootCmd = &cobra.Command{
	Use:   "cf-operator",
	Short: "cf-operator manages BOSH deployments on Kubernetes",
	Run: func(cmd *cobra.Command, args []string) {
		defer log.Sync()

		kubeConfig, err := getKubeConfig()
		if err != nil {
			log.Fatal(err)
		}
		namespace := viper.GetString("namespace")

		log.Infof("Starting cf-operator with namespace %s", namespace)

		mgr, err := operator.NewManager(log, kubeConfig, manager.Options{Namespace: namespace})
		if err != nil {
			log.Fatal(err)
		}

		log.Fatal(mgr.Start(signals.SetupSignalHandler()))
	},
}

// Execute the root command, runs the server
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		golog.Fatal(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringP("kubeconfig", "c", "", "Path to a kubeconfig, not required in-cluster")
	rootCmd.PersistentFlags().StringP("master", "m", "", "Kubernetes API server address")
	rootCmd.PersistentFlags().StringP("namespace", "n", "default", "Namespace to watch for BOSH deployments")
	viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
	viper.BindPFlag("masterURL", rootCmd.PersistentFlags().Lookup("master"))
	viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindEnv("kubeconfig")
	viper.BindEnv("namespace", "CFO_NAMESPACE")
	viper.BindEnv("masterURL", "CFO_MASTER_URL")
}

// initConfig is executed before running commands
func initConfig() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}
	log = logger.Sugar()
}

func getKubeConfig() (*rest.Config, error) {
	kubeconfig := viper.GetString("kubeconfig")
	masterURL := viper.GetString("masterURL")

	if len(kubeconfig) > 0 {
		return clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	}

	// If no explicit location, try the in-cluster config
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}

	// If no in-cluster config, try the default location in the user's home directory
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags(
			"", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not locate a kubeconfig")
}
