package cmd

import (
	"fmt"
	golog "log"
	"os"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	kubeConfig "code.cloudfoundry.org/cf-operator/pkg/kube/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/version"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
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
		log = newLogger(zap.AddCallerSkip(1))
		defer log.Sync()

		restConfig, err := kubeConfig.NewGetter(log).Get(viper.GetString("kubeconfig"))
		if err != nil {
			log.Fatal(err)
		}
		if err := kubeConfig.NewChecker(log).Check(restConfig); err != nil {
			log.Fatal(err)
		}

		cfOperatorNamespace := viper.GetString("cf-operator-namespace")
		manifest.DockerImageOrganization = viper.GetString("docker-image-org")
		manifest.DockerImageRepository = viper.GetString("docker-image-repository")
		manifest.DockerImageTag = viper.GetString("docker-image-tag")

		log.Infof("Starting cf-operator %s with namespace %s", version.Version, cfOperatorNamespace)
		log.Infof("cf-operator docker image: %s", manifest.GetOperatorDockerImage())

		operatorWebhookHost := viper.GetString("operator-webhook-host")
		operatorWebhookPort := viper.GetInt32("operator-webhook-port")

		if operatorWebhookHost == "" {
			log.Fatal("required flag 'operator-webhook-host' not set (env variable: CF_OPERATOR_WEBHOOK_HOST)")
		}

		config := &config.Config{
			CtxTimeOut:        10 * time.Second,
			Namespace:         cfOperatorNamespace,
			WebhookServerHost: operatorWebhookHost,
			WebhookServerPort: operatorWebhookPort,
			Fs:                afero.NewOsFs(),
		}
		ctx := ctxlog.NewParentContext(log)

		mgr, err := operator.NewManager(ctx, config, restConfig, manager.Options{Namespace: cfOperatorNamespace})
		if err != nil {
			log.Fatal(err)
		}

		log.Fatal(mgr.Start(signals.SetupSignalHandler()))
	},
	TraverseChildren: true,
}

// NewCFOperatorCommand returns the `cf-operator` command.
func NewCFOperatorCommand() *cobra.Command {
	return rootCmd
}

// Execute the root command, runs the server
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		golog.Fatal(err)
		os.Exit(1)
	}
}

func init() {
	pf := rootCmd.PersistentFlags()

	pf.StringP("kubeconfig", "c", "", "Path to a kubeconfig, not required in-cluster")
	pf.StringP("cf-operator-namespace", "n", "default", "Namespace to watch for BOSH deployments")
	pf.StringP("docker-image-org", "o", "cfcontainerization", "Dockerhub organization that provides the operator docker image")
	pf.StringP("docker-image-repository", "r", "cf-operator", "Dockerhub repository that provides the operator docker image")
	pf.StringP("operator-webhook-host", "w", "", "Hostname/IP under which the webhook server can be reached from the cluster")
	pf.StringP("operator-webhook-port", "p", "2999", "Port the webhook server listens on")
	pf.StringP("docker-image-tag", "t", version.Version, "Tag of the operator docker image")
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("cf-operator-namespace", pf.Lookup("cf-operator-namespace"))
	viper.BindPFlag("docker-image-org", pf.Lookup("docker-image-org"))
	viper.BindPFlag("docker-image-repository", pf.Lookup("docker-image-repository"))
	viper.BindPFlag("operator-webhook-host", pf.Lookup("operator-webhook-host"))
	viper.BindPFlag("operator-webhook-port", pf.Lookup("operator-webhook-port"))
	viper.BindPFlag("docker-image-tag", rootCmd.PersistentFlags().Lookup("docker-image-tag"))

	argToEnv := map[string]string{
	"kubeconfig": "KUBECONFIG",
	"cf-operator-namespace": "CF_OPERATOR_NAMESPACE",
	"docker-image-org": "DOCKER_IMAGE_ORG",
	"docker-image-repository": "DOCKER_IMAGE_REPOSITORY",
	"operator-webhook-host": "CF_OPERATOR_WEBHOOK_HOST",
	"operator-webhook-port": "CF_OPERATOR_WEBHOOK_PORT",
	"docker-image-tag": "DOCKER_IMAGE_TAG",
	}
	
	// Add env variables to help
	AddEnvToUsage(rootCmd, argToEnv)
}

// newLogger returns a new zap logger
func newLogger(options ...zap.Option) *zap.SugaredLogger {

	logger, err := zap.NewDevelopment(options...)
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}
	return logger.Sugar()
}

// AddEnvToUsage adds env variables to help
func AddEnvToUsage(cfOperatorCommand *cobra.Command, argToEnv map[string]string) {
	flagSet := make(map[string]bool)

	for arg, env := range argToEnv {
		viper.BindEnv(arg, env)
		flag := cfOperatorCommand.Flag(arg)

		if flag != nil {
			flagSet[flag.Name] = true
			// add environment variable to the description
			flag.Usage = fmt.Sprintf("(%s) %s", env, flag.Usage)
		}
	}
}
