package environment

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" //from https://github.com/kubernetes/client-go/issues/345
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"code.cloudfoundry.org/cf-operator/testing"
	gomegaConfig "github.com/onsi/ginkgo/config"
	"github.com/onsi/gomega"
)

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
			CtxTimeOut: 10 * time.Second,
			Fs:         afero.NewOsFs(),
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
		return err
	}

	err = e.startKubeClients(e.kubeConfig)
	if err != nil {
		return err
	}

	nsTeardown, err := e.CreateNamespace(e.Namespace)
	if err != nil {
		return err
	}

	e.Teardown = func(wasFailure bool) {
		if wasFailure {
			fmt.Println("Collecting debug information...")
			out, err := exec.Command("../testing/dump_env.sh", e.Namespace).CombinedOutput()
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
		return err
	}

	e.stop = e.startOperator()
	err = helper.WaitForPort(
		"127.0.0.1",
		strconv.Itoa(int(e.Config.WebhookServerPort)),
		1*time.Minute)
	if err != nil {
		return err
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

func (e *Environment) setupCFOperator(namespace string) (err error) {
	whh, found := os.LookupEnv("CF_OPERATOR_WEBHOOK_SERVICE_HOST")
	if !found {
		return fmt.Errorf("no webhook host set. Please set CF_OPERATOR_WEBHOOK_SERVICE_HOST to the host/ip the operator runs on and try again")
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

	operatorDockerImageOrg, found := os.LookupEnv("DOCKER_IMAGE_ORG")
	if !found {
		operatorDockerImageOrg = "cfcontainerization"
	}

	operatorDockerImageRepo, found := os.LookupEnv("DOCKER_IMAGE_REPOSITORY")
	if !found {
		operatorDockerImageRepo = "cf-operator"
	}

	operatorDockerImageTag, found := os.LookupEnv("DOCKER_IMAGE_TAG")
	if !found {
		return fmt.Errorf("required environment variable DOCKER_IMAGE_TAG not set")
	}

	manifest.DockerImageOrganization = operatorDockerImageOrg
	manifest.DockerImageRepository = operatorDockerImageRepo
	manifest.DockerImageTag = operatorDockerImageTag

	loggerPath := helper.LogfilePath(fmt.Sprintf("cf-operator-tests-%d.log", e.ID))
	e.ObservedLogs, e.Log = helper.NewTestLoggerWithPath(loggerPath)
	ctx := ctxlog.NewParentContext(e.Log)
	e.mgr, err = operator.NewManager(ctx, e.Config, e.kubeConfig, manager.Options{Namespace: e.Namespace})

	return
}

func (e *Environment) startOperator() chan struct{} {
	stop := make(chan struct{})
	go e.mgr.Start(stop)
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
			return -1, err
		}
	}
	return int32(port) + int32(namespaceCounter), nil
}

// DeleteNamespace removes existing ns
func DeleteNamespace(ns string, kubeCtlCmd string) error {
	fmt.Printf("Cleaning up namespace %s \n", ns)

	_, err := RunBinary(kubeCtlCmd, "delete", "--wait=false", "--ignore-not-found", "namespace", ns)
	if err != nil {
		return err
	}

	return nil
}

// DeleteWebhook removes existing mutatingwebhookconfiguration
func DeleteWebhook(ns string, kubeCtlCmd string) error {
	webHookName := fmt.Sprintf("%s-%s", "cf-operator-hook", ns)
	fmt.Printf("Cleaning up mutatingwebhookconfiguration %s \n", webHookName)

	_, err := RunBinary(kubeCtlCmd, "delete", "--ignore-not-found", "mutatingwebhookconfiguration", webHookName)
	if err != nil {
		return err
	}

	return nil
}

// RunBinary executes a binary cmd and returns the stdOutput
func RunBinary(binaryName string, args ...string) ([]byte, error) {
	cmd := exec.Command(binaryName, args...)
	stdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return stdOutput, errors.Wrapf(err, "%s cmd, failed with the following error: %s", cmd.Args, string(stdOutput))
	}
	return stdOutput, nil
}
