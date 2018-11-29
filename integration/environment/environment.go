package environment

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/operator"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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
	Catalog
	mgr        manager.Manager
	kubeConfig *rest.Config
	log        *zap.SugaredLogger
	stop       chan struct{}

	LogRecorded *observer.ObservedLogs
	Namespace   string
}

// NewEnvironment returns a new struct
func NewEnvironment() *Environment {
	return &Environment{
		Namespace: "",
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
	return func() {
		if e.stop != nil {
			close(e.stop)
		}
	}, nil
}

// AllLogMessages returns only the message part of existing logs to aid in debugging
func (e *Environment) AllLogMessages() (msgs []string) {
	for _, m := range e.LogRecorded.All() {
		msgs = append(msgs, m.Message)
	}

	return
}

func (e *Environment) setupCFOperator() (err error) {
	ns, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		ns = "default"
	}
	e.Namespace = ns

	var core zapcore.Core
	core, e.LogRecorded = observer.New(zapcore.InfoLevel)
	e.log = zap.New(core).Sugar()

	err = e.setupKube()
	if err != nil {
		return
	}

	e.mgr, err = operator.NewManager(e.log, e.kubeConfig, manager.Options{Namespace: e.Namespace})
	return
}

func (e *Environment) setupKube() (err error) {
	location := os.Getenv("KUBE_CONFIG")
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
