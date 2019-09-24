package cmd

import (
	"fmt"
	golog "log"
	"os"
	"time"

	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // from https://github.com/kubernetes/client-go/issues/345
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	kubeConfig "code.cloudfoundry.org/cf-operator/pkg/kube/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/version"
)

const (
	cfFailedMessage = "cf-operator command failed."
	// Port on which the controller-runtime manager listens
	managerPort = 2999
)

var (
	log              *zap.SugaredLogger
	debugGracePeriod = time.Second * 5
)

var rootCmd = &cobra.Command{
	Use:   "cf-operator",
	Short: "cf-operator manages BOSH deployments on Kubernetes",
	RunE: func(cmd *cobra.Command, args []string) error {
		log = newLogger(zap.AddCallerSkip(1))
		defer log.Sync()

		restConfig, err := kubeConfig.NewGetter(log).Get(viper.GetString("kubeconfig"))
		if err != nil {
			return errors.Wrapf(err, "%s Couldn't fetch Kubeconfig. Ensure kubeconfig is present to continue.", cfFailedMessage)
		}
		if err := kubeConfig.NewChecker(log).Check(restConfig); err != nil {
			return errors.Wrapf(err, "%s Couldn't check Kubeconfig. Ensure kubeconfig is correct to continue.", cfFailedMessage)
		}

		cfOperatorNamespace := viper.GetString("cf-operator-namespace")
		converter.SetupOperatorDockerImage(
			viper.GetString("docker-image-org"),
			viper.GetString("docker-image-repository"),
			viper.GetString("docker-image-tag"),
		)

		log.Infof("Starting cf-operator %s with namespace %s", version.Version, cfOperatorNamespace)
		log.Infof("cf-operator docker image: %s", converter.GetOperatorDockerImage())

		serviceHost := viper.GetString("operator-webhook-service-host")
		// Port on which the cf operator webhook kube service listens to.
		servicePort := viper.GetInt32("operator-webhook-service-port")
		provider := viper.GetString("provider")

		if serviceHost == "" && provider == "" {
			log.Fatalf("%s operator-webhook-service-host flag is not set (env variable: CF_OPERATOR_WEBHOOK_SERVICE_HOST).", cfFailedMessage)
		}

		config := config.NewConfig(
			cfOperatorNamespace,
			provider,
			serviceHost,
			servicePort,
			afero.NewOsFs(),
			viper.GetInt("max-boshdeployment-workers"),
			viper.GetInt("max-extendedjob-workers"),
			viper.GetInt("max-extendedsecret-workers"),
			viper.GetInt("max-extendedstatefulset-workers"),
			viper.GetBool("apply-crd"),
		)
		ctx := ctxlog.NewParentContext(log)

		if viper.GetBool("apply-crd") {
			ctxlog.Info(ctx, "Applying CRDs...")
			err := operator.ApplyCRDs(restConfig)
			if err != nil {
				return errors.Wrap(err, "failed to apply crds")
			}
		}

		mgr, err := operator.NewManager(ctx, config, restConfig, manager.Options{
			Namespace:          cfOperatorNamespace,
			MetricsBindAddress: "0",
			LeaderElection:     false,
			Port:               managerPort,
			Host:               "0.0.0.0",
		})
		if err != nil {
			return errors.Wrapf(err, cfFailedMessage)
		}

		ctxlog.Info(ctx, "Waiting for configurations to be applied into a BOSHDeployment resource...")

		err = mgr.Start(signals.SetupSignalHandler())
		if err != nil {
			return errors.Wrapf(err, "%s Failed to start cf-operator manager", cfFailedMessage)
		}
		return nil
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
	pf.StringP("log-level", "l", "debug", "Only print log messages from this level onward")
	pf.StringP("cf-operator-namespace", "n", "default", "Namespace to watch for BOSH deployments")
	pf.StringP("docker-image-org", "o", "cfcontainerization", "Dockerhub organization that provides the operator docker image")
	pf.StringP("docker-image-repository", "r", "cf-operator", "Dockerhub repository that provides the operator docker image")
	pf.StringP("operator-webhook-service-host", "w", "", "Hostname/IP under which the webhook server can be reached from the cluster")
	pf.StringP("operator-webhook-service-port", "p", "2999", "Port the webhook server listens on")
	pf.StringP("docker-image-tag", "t", version.Version, "Tag of the operator docker image")
	pf.StringP("provider", "x", "", "Cloud Provider where cf-operator is being deployed")
	pf.Int("max-boshdeployment-workers", 1, "Maximum of number concurrently running BOSHDeployment controller")
	pf.Int("max-extendedjob-workers", 1, "Maximum of number concurrently running ExtendedJob controller")
	pf.Int("max-extendedsecret-workers", 5, "Maximum of number concurrently running ExtendedSecret controller")
	pf.Int("max-extendedstatefulset-workers", 1, "Maximum of number concurrently running ExtendedStatefulSet controller")
	pf.Bool("apply-crd", true, "If true, apply CRDs on start")
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("log-level", pf.Lookup("log-level"))
	viper.BindPFlag("cf-operator-namespace", pf.Lookup("cf-operator-namespace"))
	viper.BindPFlag("docker-image-org", pf.Lookup("docker-image-org"))
	viper.BindPFlag("docker-image-repository", pf.Lookup("docker-image-repository"))
	viper.BindPFlag("operator-webhook-service-host", pf.Lookup("operator-webhook-service-host"))
	viper.BindPFlag("operator-webhook-service-port", pf.Lookup("operator-webhook-service-port"))
	viper.BindPFlag("provider", pf.Lookup("provider"))
	viper.BindPFlag("docker-image-tag", rootCmd.PersistentFlags().Lookup("docker-image-tag"))
	viper.BindPFlag("max-boshdeployment-workers", pf.Lookup("max-boshdeployment-workers"))
	viper.BindPFlag("max-extendedjob-workers", pf.Lookup("max-extendedjob-workers"))
	viper.BindPFlag("max-extendedsecret-workers", pf.Lookup("max-extendedsecret-workers"))
	viper.BindPFlag("max-extendedstatefulset-workers", rootCmd.PersistentFlags().Lookup("max-extendedstatefulset-workers"))
	viper.BindPFlag("apply-crd", rootCmd.PersistentFlags().Lookup("apply-crd"))

	argToEnv := map[string]string{
		"kubeconfig":                      "KUBECONFIG",
		"log-level":                       "LOG_LEVEL",
		"cf-operator-namespace":           "CF_OPERATOR_NAMESPACE",
		"docker-image-org":                "DOCKER_IMAGE_ORG",
		"docker-image-repository":         "DOCKER_IMAGE_REPOSITORY",
		"operator-webhook-service-host":   "CF_OPERATOR_WEBHOOK_SERVICE_HOST",
		"operator-webhook-service-port":   "CF_OPERATOR_WEBHOOK_SERVICE_PORT",
		"provider":                        "PROVIDER",
		"docker-image-tag":                "DOCKER_IMAGE_TAG",
		"max-boshdeployment-workers":      "MAX_BOSHDEPLOYMENT_WORKERS",
		"max-extendedjob-workers":         "MAX_EXTENDEDJOB_WORKERS",
		"max-extendedsecret-workers":      "MAX_EXTENDEDSECRET_WORKERS",
		"max-extendedstatefulset-workers": "MAX_EXTENDEDSTATEFULSET_WORKERS",
		"apply-crd":                       "APPLY_CRD",
	}

	// Add env variables to help
	AddEnvToUsage(rootCmd, argToEnv)

	// Do not display cmd usage and errors
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
}

// newLogger returns a new zap logger
func newLogger(options ...zap.Option) *zap.SugaredLogger {
	level := viper.GetString("log-level")
	l := zap.DebugLevel
	l.Set(level)

	cfg := zap.NewDevelopmentConfig()
	cfg.Development = false
	cfg.Level = zap.NewAtomicLevelAt(l)
	logger, err := cfg.Build(options...)
	if err != nil {
		golog.Fatalf("cannot initialize ZAP logger: %v", err)
	}

	// Make controller-runtime log using our logger
	crlog.SetLogger(zapr.NewLogger(logger))

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
