package cmd

import (
	"fmt"
	golog "log"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"

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
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/cf-operator/version"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	kubeConfig "code.cloudfoundry.org/quarks-utils/pkg/kubeconfig"
)

const (
	// Port on which the controller-runtime manager listens
	managerPort = 2999
)

var (
	log              *zap.SugaredLogger
	debugGracePeriod = time.Second * 5
)

func wrapError(err error, msg string) error {
	return errors.Wrap(err, "cf-operator command failed. "+msg)
}

var rootCmd = &cobra.Command{
	Use:   "cf-operator",
	Short: "cf-operator manages BOSH deployments on Kubernetes",
	RunE: func(cmd *cobra.Command, args []string) error {
		log = newLogger(zap.AddCallerSkip(1))
		defer log.Sync()

		restConfig, err := kubeConfig.NewGetter(log).Get(viper.GetString("kubeconfig"))
		if err != nil {
			return wrapError(err, "Couldn't fetch Kubeconfig. Ensure kubeconfig is present to continue.")
		}
		if err := kubeConfig.NewChecker(log).Check(restConfig); err != nil {
			return wrapError(err, "Couldn't check Kubeconfig. Ensure kubeconfig is correct to continue.")
		}

		cfOperatorNamespace := viper.GetString("cf-operator-namespace")
		watchNamespace := viper.GetString("watch-namespace")
		if watchNamespace == "" {
			log.Infof("No watch namespace defined. Falling back to the operator namespace.")
			watchNamespace = cfOperatorNamespace
		}

		err = converter.SetupOperatorDockerImage(
			viper.GetString("docker-image-org"),
			viper.GetString("docker-image-repository"),
			viper.GetString("docker-image-tag"),
			viper.GetString("docker-image-pull-policy"),
		)
		if err != nil {
			return wrapError(err, "Couldn't parse cf-operator docker image reference.")
		}

		manifest.SetupBoshDNSDockerImage(viper.GetString("bosh-dns-docker-image"))

		log.Infof("Starting cf-operator %s with namespace %s", version.Version, watchNamespace)
		log.Infof("cf-operator docker image: %s", converter.GetOperatorDockerImage())

		serviceHost := viper.GetString("operator-webhook-service-host")
		// Port on which the cf operator webhook kube service listens to.
		servicePort := viper.GetInt32("operator-webhook-service-port")
		useServiceRef := viper.GetBool("operator-webhook-use-service-reference")

		if serviceHost == "" && !useServiceRef {
			return wrapError(errors.New("couldn't determine webhook server"), "operator-webhook-service-host flag is not set (env variable: CF_OPERATOR_WEBHOOK_SERVICE_HOST)")
		}

		config := config.NewConfig(
			watchNamespace,
			cfOperatorNamespace,
			viper.GetInt("ctx-timeout"),
			useServiceRef,
			serviceHost,
			servicePort,
			afero.NewOsFs(),
			viper.GetInt("max-boshdeployment-workers"),
			viper.GetInt("max-extendedjob-workers"),
			viper.GetInt("max-extendedsecret-workers"),
			viper.GetInt("max-extendedstatefulset-workers"),
		)
		ctx := ctxlog.NewParentContext(log)

		if viper.GetBool("apply-crd") {
			ctxlog.Info(ctx, "Applying CRDs...")
			err := operator.ApplyCRDs(restConfig)
			if err != nil {
				return wrapError(err, "Couldn't apply CRDs.")
			}
		}

		mgr, err := operator.NewManager(ctx, config, restConfig, manager.Options{
			Namespace:          watchNamespace,
			MetricsBindAddress: "0",
			LeaderElection:     false,
			Port:               managerPort,
			Host:               "0.0.0.0",
		})
		if err != nil {
			return wrapError(err, "Failed to create new manager.")
		}

		ctxlog.Info(ctx, "Waiting for configurations to be applied into a BOSHDeployment resource...")

		err = mgr.Start(signals.SetupSignalHandler())
		if err != nil {
			return wrapError(err, "Failed to start cf-operator manager.")
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

	pf.Bool("apply-crd", true, "If true, apply CRDs on start")
	pf.Int("ctx-timeout", 30, "context timeout for each k8s API request in seconds")
	pf.StringP("cf-operator-namespace", "n", "default", "The operator namespace")
	pf.StringP("docker-image-org", "o", "cfcontainerization", "Dockerhub organization that provides the operator docker image")
	pf.StringP("docker-image-repository", "r", "cf-operator", "Dockerhub repository that provides the operator docker image")
	pf.StringP("docker-image-tag", "t", version.Version, "Tag of the operator docker image")
	pf.StringP("docker-image-pull-policy", "", string(corev1.PullAlways), "Image pull policy")
	pf.StringP("bosh-dns-docker-image", "", "coredns/coredns:1.6.3", "The docker image used for emulating bosh DNS (a CoreDNS image)")
	pf.StringP("kubeconfig", "c", "", "Path to a kubeconfig, not required in-cluster")
	pf.StringP("log-level", "l", "debug", "Only print log messages from this level onward")
	pf.Int("max-boshdeployment-workers", 1, "Maximum number of workers concurrently running BOSHDeployment controller")
	pf.Int("max-extendedjob-workers", 1, "Maximum number of workers concurrently running ExtendedJob controller")
	pf.Int("max-extendedsecret-workers", 5, "Maximum number of workers concurrently running ExtendedSecret controller")
	pf.Int("max-extendedstatefulset-workers", 1, "Maximum number of workers concurrently running ExtendedStatefulSet controller")
	pf.StringP("operator-webhook-service-host", "w", "", "Hostname/IP under which the webhook server can be reached from the cluster")
	pf.StringP("operator-webhook-service-port", "p", "2999", "Port the webhook server listens on")
	pf.BoolP("operator-webhook-use-service-reference", "x", false, "If true the webhook service is targetted using a service reference instead of a URL")
	pf.StringP("watch-namespace", "", "", "Namespace to watch for BOSH deployments")

	viper.BindPFlag("apply-crd", rootCmd.PersistentFlags().Lookup("apply-crd"))
	viper.BindPFlag("ctx-timeout", pf.Lookup("ctx-timeout"))
	viper.BindPFlag("cf-operator-namespace", pf.Lookup("cf-operator-namespace"))
	viper.BindPFlag("docker-image-org", pf.Lookup("docker-image-org"))
	viper.BindPFlag("docker-image-repository", pf.Lookup("docker-image-repository"))
	viper.BindPFlag("docker-image-tag", rootCmd.PersistentFlags().Lookup("docker-image-tag"))
	viper.BindPFlag("docker-image-pull-policy", rootCmd.PersistentFlags().Lookup("docker-image-pull-policy"))
	viper.BindPFlag("bosh-dns-docker-image", rootCmd.PersistentFlags().Lookup("bosh-dns-docker-image"))
	viper.BindPFlag("kubeconfig", pf.Lookup("kubeconfig"))
	viper.BindPFlag("log-level", pf.Lookup("log-level"))
	viper.BindPFlag("max-boshdeployment-workers", pf.Lookup("max-boshdeployment-workers"))
	viper.BindPFlag("max-extendedjob-workers", pf.Lookup("max-extendedjob-workers"))
	viper.BindPFlag("max-extendedsecret-workers", pf.Lookup("max-extendedsecret-workers"))
	viper.BindPFlag("max-extendedstatefulset-workers", rootCmd.PersistentFlags().Lookup("max-extendedstatefulset-workers"))
	viper.BindPFlag("operator-webhook-service-host", pf.Lookup("operator-webhook-service-host"))
	viper.BindPFlag("operator-webhook-service-port", pf.Lookup("operator-webhook-service-port"))
	viper.BindPFlag("operator-webhook-use-service-reference", pf.Lookup("operator-webhook-use-service-reference"))
	viper.BindPFlag("watch-namespace", pf.Lookup("watch-namespace"))

	argToEnv := map[string]string{
		"apply-crd":                              "APPLY_CRD",
		"ctx-timeout":                            "CTX_TIMEOUT",
		"cf-operator-namespace":                  "CF_OPERATOR_NAMESPACE",
		"docker-image-org":                       "DOCKER_IMAGE_ORG",
		"docker-image-repository":                "DOCKER_IMAGE_REPOSITORY",
		"docker-image-tag":                       "DOCKER_IMAGE_TAG",
		"docker-image-pull-policy":               "DOCKER_IMAGE_PULL_POLICY",
		"bosh-dns-docker-image":                  "BOSH_DNS_DOCKER_IMAGE",
		"kubeconfig":                             "KUBECONFIG",
		"log-level":                              "LOG_LEVEL",
		"max-boshdeployment-workers":             "MAX_BOSHDEPLOYMENT_WORKERS",
		"max-extendedjob-workers":                "MAX_EXTENDEDJOB_WORKERS",
		"max-extendedsecret-workers":             "MAX_EXTENDEDSECRET_WORKERS",
		"max-extendedstatefulset-workers":        "MAX_EXTENDEDSTATEFULSET_WORKERS",
		"operator-webhook-service-host":          "CF_OPERATOR_WEBHOOK_SERVICE_HOST",
		"operator-webhook-service-port":          "CF_OPERATOR_WEBHOOK_SERVICE_PORT",
		"operator-webhook-use-service-reference": "CF_OPERATOR_WEBHOOK_USE_SERVICE_REFERENCE",
		"watch-namespace":                        "WATCH_NAMESPACE",
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
