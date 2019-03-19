package cmd

import (
	"fmt"
	golog "log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"code.cloudfoundry.org/cf-operator/version"

	"github.com/spf13/afero"
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
		manifest.DockerOrganization = viper.GetString("docker-image-org")
		manifest.DockerRepository = viper.GetString("docker-image-repository")
		manifest.DockerTag = viper.GetString("docker-image-tag")

		log.Infof("Starting cf-operator %s with namespace %s", version.Version, namespace)
		log.Infof("cf-operator docker image: %s", manifest.GetOperatorDockerImage())

		webhookHost := viper.GetString("operator-webhook-host")
		webhookPort := viper.GetInt32("operator-webhook-port")

		if webhookHost == "" {
			log.Fatal("required flag 'operator-webhook-host' not set (env variable: OPERATOR_WEBHOOK_HOST)")
		}

		ctrsConfig := &context.Config{ //Set the context to be TODO
			CtxTimeOut:        10 * time.Second,
			CtxType:           context.NewBackgroundContext(),
			Namespace:         namespace,
			WebhookServerHost: webhookHost,
			WebhookServerPort: webhookPort,
			Fs:                afero.NewOsFs(),
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
	pf := rootCmd.PersistentFlags()

	pf.StringP("kubeconfig", "c", "", "Path to a kubeconfig, not required in-cluster")
	pf.StringP("namespace", "n", "default", "Namespace to watch for BOSH deployments")
	pf.StringP("docker-image-org", "o", "cfcontainerization", "Dockerhub organization that provides the operator docker image")
	pf.StringP("docker-image-repository", "r", "cf-operator", "Dockerhub repository that provides the operator docker image")
	pf.StringP("operator-webhook-host", "w", "", "Hostname/IP under which the webhook server can be reached from the cluster")
	pf.StringP("operator-webhook-port", "p", "2999", "Port the webhook server listens on")
	pf.StringP("docker-image-tag", "t", version.Version, "Tag of the operator docker image")
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("namespace", pf.Lookup("namespace"))
	viper.BindPFlag("docker-image-org", pf.Lookup("docker-image-org"))
	viper.BindPFlag("docker-image-repository", pf.Lookup("docker-image-repository"))
	viper.BindPFlag("operator-webhook-host", pf.Lookup("operator-webhook-host"))
	viper.BindPFlag("operator-webhook-port", pf.Lookup("operator-webhook-port"))
	viper.BindPFlag("docker-image-tag", rootCmd.PersistentFlags().Lookup("docker-image-tag"))
	viper.BindEnv("kubeconfig")
	viper.BindEnv("namespace", "CFO_NAMESPACE")
	viper.BindEnv("docker-image-org", "DOCKER_IMAGE_ORG")
	viper.BindEnv("docker-image-repository", "DOCKER_IMAGE_REPOSITORY")
	viper.BindEnv("operator-webhook-host", "OPERATOR_WEBHOOK_HOST")
	viper.BindEnv("operator-webhook-port", "OPERATOR_WEBHOOK_PORT")
	viper.BindEnv("docker-image-tag", "DOCKER_IMAGE_TAG")
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
