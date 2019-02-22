package cmd

import (
	"fmt"
	golog "log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
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

		ctrsConfig := &context.Config{ //Set the context to be TODO
			CtxTimeOut: 10 * time.Second,
			CtxType:    context.NewBackgroundContext(),
		}
		mgr, err := operator.NewManager(log, ctrsConfig, kubeConfig, manager.Options{Namespace: namespace})
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
	rootCmd.PersistentFlags().StringP("namespace", "n", "default", "Namespace to watch for BOSH deployments")
	viper.BindPFlag("kubeconfig", rootCmd.PersistentFlags().Lookup("kubeconfig"))
	viper.BindPFlag("namespace", rootCmd.PersistentFlags().Lookup("namespace"))
	viper.BindEnv("kubeconfig")
	viper.BindEnv("namespace", "CFO_NAMESPACE")
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

	if len(kubeconfig) > 0 {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
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
