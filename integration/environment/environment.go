package environment

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-logr/zapr"
	gomegaConfig "github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" //from https://github.com/kubernetes/client-go/issues/345
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"code.cloudfoundry.org/cf-operator/testing"
)

const envFailedMessage = "Integration env setup failed."

// StopFunc is used to clean up the environment
type StopFunc func()

// Environment starts our operator and handles interaction with the k8s
// cluster used in the tests
type Environment struct {
	Machine

	testing.Catalog
	mgr        manager.Manager
	kubeConfig *rest.Config
	stop       chan struct{}

	ID           int
	Teardown     func(wasFailure bool)
	Log          *zap.SugaredLogger
	Config       *config.Config
	ObservedLogs *observer.ObservedLogs
	Namespace    string
}

var (
	namespaceCounter int32
)

// SetupNamespace creates a namespace that's meant to be used for one
// test, and then destroyed
func SetupNamespace() *Environment {
	atomic.AddInt32(&namespaceCounter, 1)
	namespaceID := gomegaConfig.GinkgoConfig.ParallelNode*100 + int(namespaceCounter)

	env := newEnvironment(namespaceID)
	err := env.setup()
	if err != nil {
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}

	return env
}

// newEnvironment returns a new struct
func newEnvironment(namespaceCounter int) *Environment {
	return &Environment{
		ID:        namespaceCounter,
		Namespace: getNamespace(namespaceCounter),
		Config: &config.Config{
			CtxTimeOut:           10 * time.Second,
			MeltdownDuration:     1 * time.Second,
			MeltdownRequeueAfter: 500 * time.Millisecond,
			Fs:                   afero.NewOsFs(),
		},
		Machine: Machine{
			pollTimeout:  300 * time.Second,
			pollInterval: 500 * time.Millisecond,
		},
	}
}

// Setup prepares the test environment by loading config and finally starting the operator
func (e *Environment) setup() error {
	err := e.setupKube()
	if err != nil {
		return errors.Wrapf(err, "%s Setting up Kube failed.", envFailedMessage)
	}

	err = e.startKubeClients(e.kubeConfig)
	if err != nil {
		return errors.Wrapf(err, "%s Starting kube clients failed.", envFailedMessage)
	}

	nsTeardown, err := e.CreateNamespace(e.Namespace)
	if err != nil {
		return errors.Wrapf(err, "Integration setup failed. Creating namespace %s failed", e.Namespace)
	}

	e.Teardown = func(wasFailure bool) {
		if wasFailure {
			fmt.Println("Collecting debug information...")

			// try to find our dump_env script
			n := 1
			_, filename, _, _ := runtime.Caller(1)
			if idx := strings.Index(filename, "integration/"); idx >= 0 {
				n = strings.Count(filename[idx:], "/")
			}
			var dots []string
			for i := 0; i < n; i++ {
				dots = append(dots, "..")
			}
			dumpCmd := path.Join(append(dots, "testing/dump_env.sh")...)

			out, err := exec.Command(dumpCmd, e.Namespace).CombinedOutput()
			if err != nil {
				fmt.Println("Failed to run the `dump_env.sh` script", err)
			}
			fmt.Println(string(out))
		}

		err := nsTeardown()
		if err != nil {
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		}

		if e.stop != nil {
			close(e.stop)
		}

		e.removeWebhookCache()
	}

	err = e.setupCFOperator(e.Namespace)
	if err != nil {
		return errors.Wrapf(err, "%s Setting up CF Operator failed.", envFailedMessage)
	}

	e.stop = e.startOperator()
	err = helper.WaitForPort(
		"127.0.0.1",
		strconv.Itoa(int(e.Config.WebhookServerPort)),
		1*time.Minute)
	if err != nil {
		return errors.Wrapf(err, "Integration setup failed. Waiting for port %d failed.", e.Config.WebhookServerPort)
	}

	return nil
}

// removeWebhookCache removes the local webhook config temp folder
func (e *Environment) removeWebhookCache() {
	os.RemoveAll(path.Join(controllers.WebhookConfigDir, controllers.WebhookConfigPrefix+getNamespace(e.ID)))
}

// FlushLog flushes the zap log
func (e *Environment) FlushLog() error {
	return e.Log.Sync()
}

// AllLogMessages returns only the message part of existing logs to aid in debugging
func (e *Environment) AllLogMessages() (msgs []string) {
	for _, m := range e.ObservedLogs.All() {
		msgs = append(msgs, m.Message)
	}

	return
}

// NodeIP returns a public IP of a node
func (e *Environment) NodeIP() (string, error) {
	nodes, err := e.Clientset.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return "", errors.Wrap(err, "Getting the list of nodes")
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("Got an empty list of nodes")
	}

	addresses := nodes.Items[0].Status.Addresses
	if len(addresses) == 0 {
		return "", fmt.Errorf("Node has an empty list of addresses")
	}

	return addresses[0].Address, nil
}

func (e *Environment) setupKube() (err error) {
	location := os.Getenv("KUBECONFIG")
	if location == "" {
		location = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	e.kubeConfig, err = clientcmd.BuildConfigFromFlags("", location)
	if err != nil {
		log.Printf("INFO: cannot use kube config: %s\n", err)
		e.kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			return
		}
	}

	return
}

func (e *Environment) startKubeClients(kubeConfig *rest.Config) (err error) {
	e.Clientset, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return
	}
	e.VersionedClientset, err = versioned.NewForConfig(kubeConfig)
	return
}

func (e *Environment) setupCFOperator(namespace string) error {
	var err error
	whh, found := os.LookupEnv("CF_OPERATOR_WEBHOOK_SERVICE_HOST")
	if !found {
		return errors.Errorf("Please set CF_OPERATOR_WEBHOOK_SERVICE_HOST to the host/ip the operator runs on and try again")
	}
	e.Config.WebhookServerHost = whh

	port, err := getWebhookServicePort(e.ID)
	if err != nil {
		return err
	}

	sshUser, shouldForwardPort := os.LookupEnv("ssh_server_user")
	if shouldForwardPort {
		go func() {
			cmd := exec.Command(
				"ssh", "-fNT", "-i", "/tmp/cf-operator-tunnel-identity", "-o",
				"UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no", "-R",
				fmt.Sprintf("%s:%[2]d:localhost:%[2]d", whh, port),
				fmt.Sprintf("%s@%s", sshUser, whh))

			stdOutput, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("SSH TUNNEL FAILED: %f\nOUTPUT: %s", err, string(stdOutput))
			}
		}()
	}

	e.Config.WebhookServerPort = port

	e.Namespace = namespace
	e.Config.Namespace = namespace

	dockerImageOrg, found := os.LookupEnv("DOCKER_IMAGE_ORG")
	if !found {
		dockerImageOrg = "cfcontainerization"
	}

	dockerImageRepo, found := os.LookupEnv("DOCKER_IMAGE_REPOSITORY")
	if !found {
		dockerImageRepo = "cf-operator"
	}

	dockerImageTag, found := os.LookupEnv("DOCKER_IMAGE_TAG")
	if !found {
		return errors.Errorf("required environment variable DOCKER_IMAGE_TAG not set")
	}

	err = converter.SetupOperatorDockerImage(dockerImageOrg, dockerImageRepo, dockerImageTag)
	if err != nil {
		return err
	}

	loggerPath := helper.LogfilePath(fmt.Sprintf("cf-operator-tests-%d.log", e.ID))
	e.ObservedLogs, e.Log = helper.NewTestLoggerWithPath(loggerPath)
	crlog.SetLogger(zapr.NewLogger(e.Log.Desugar()))

	ctx := ctxlog.NewParentContext(e.Log)
	e.mgr, err = operator.NewManager(ctx, e.Config, e.kubeConfig, manager.Options{
		Namespace:          e.Namespace,
		MetricsBindAddress: "0",
		LeaderElection:     false,
		Port:               int(e.Config.WebhookServerPort),
		Host:               "0.0.0.0",
	})

	return err
}

func (e *Environment) startOperator() chan struct{} {
	stop := make(chan struct{})
	go func() {
		err := e.mgr.Start(stop)
		if err != nil {
			panic(err)
		}
	}()
	return stop
}

func getNamespace(namespaceCounter int) string {
	ns, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		ns = "default"
	}
	return ns + "-" + strconv.Itoa(int(namespaceCounter))
}

func getWebhookServicePort(namespaceCounter int) (int32, error) {
	port := int64(40000)
	portString, found := os.LookupEnv("CF_OPERATOR_WEBHOOK_SERVICE_PORT")
	if found {
		var err error
		port, err = strconv.ParseInt(portString, 10, 32)
		if err != nil {
			return -1, errors.Wrapf(err, "Parsing portSting %s failed", portString)
		}
	}
	return int32(port) + int32(namespaceCounter), nil
}
