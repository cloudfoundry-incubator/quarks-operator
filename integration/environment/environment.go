package environment

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/testhelper"

	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"code.cloudfoundry.org/cf-operator/testing"
	"github.com/spf13/afero"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" //from https://github.com/kubernetes/client-go/issues/345

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

	Log          *zap.SugaredLogger
	CtrsConfig   *context.Config
	ObservedLogs *observer.ObservedLogs
	Namespace    string
}

// NewEnvironment returns a new struct
func NewEnvironment() *Environment {
	return &Environment{
		Namespace: "",
		CtrsConfig: &context.Config{ //Set the context to be TODO
			CtxTimeOut: 10 * time.Second,
			CtxType:    context.NewBackgroundContext(),
			Fs:         afero.NewOsFs(),
		},
		Machine: Machine{
			pollTimeout:  30 * time.Second,
			pollInterval: 500 * time.Millisecond,
		},
	}
}

// Setup prepares the test environment by loading config and finally starting the operator
func (e *Environment) Setup() (StopFunc, error) {
	err := e.setupCFOperator()
	if err != nil {
		return nil, err
	}

	err = e.startClients(e.kubeConfig)
	if err != nil {
		return nil, err
	}

	e.stop = e.startOperator()

	err = testhelper.WaitForPort(
		"127.0.0.1",
		strconv.Itoa(int(e.CtrsConfig.WebhookServerPort)),
		1*time.Minute)

	if err != nil {
		return nil, err
	}

	return func() {
		if e.stop != nil {
			close(e.stop)
		}
	}, nil
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

func (e *Environment) setupCFOperator() (err error) {
	whh, found := os.LookupEnv("OPERATOR_WEBHOOK_HOST")
	if !found {
		return fmt.Errorf("no webhook host set. Please set OPERATOR_WEBHOOK_HOST to the host/ip the operator runs on and try again")
	}
	e.CtrsConfig.WebhookServerHost = whh

	whp := int32(2999)
	portString, found := os.LookupEnv("OPERATOR_WEBHOOK_PORT")
	if found {
		port, err := strconv.ParseInt(portString, 10, 32)
		if err != nil {
			return err
		}
		whp = int32(port)
	}
	e.CtrsConfig.WebhookServerPort = whp

	ns, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		ns = "default"
	}

	e.Namespace = ns
	e.CtrsConfig.Namespace = ns

	e.ObservedLogs, e.Log = testing.NewTestLogger()

	err = e.setupKube()
	if err != nil {
		return
	}

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
		operatorDockerImageTag = "latest"
	}

	manifest.DockerOrganization = operatorDockerImageOrg
	manifest.DockerRepository = operatorDockerImageRepo
	manifest.DockerTag = operatorDockerImageTag

	e.mgr, err = operator.NewManager(e.Log, e.CtrsConfig, e.kubeConfig, manager.Options{Namespace: e.Namespace})

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

func (e *Environment) startClients(kubeConfig *rest.Config) (err error) {
	e.Clientset, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return
	}
	e.VersionedClientset, err = versioned.NewForConfig(kubeConfig)
	return
}

func (e *Environment) startOperator() chan struct{} {
	stop := make(chan struct{})
	go e.mgr.Start(stop)
	return stop
}
