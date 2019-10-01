package environment

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	gomegaConfig "github.com/onsi/ginkgo/config"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" //from https://github.com/kubernetes/client-go/issues/345
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/testing"
)

// Environment starts our operator and handles interaction with the k8s
// cluster used in the tests
type Environment struct {
	Machine
	testing.Catalog

	mgr manager.Manager

	ID           int
	Teardown     func(wasFailure bool)
	KubeConfig   *rest.Config
	Log          *zap.SugaredLogger
	Config       *config.Config
	ObservedLogs *observer.ObservedLogs
	Namespace    string
	Stop         chan struct{}
}

var (
	namespaceCounter int32
)

// KubeConfig returns a kube config for this environment
func KubeConfig() (*rest.Config, error) {
	location := os.Getenv("KUBECONFIG")
	if location == "" {
		location = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}

	config, err := clientcmd.BuildConfigFromFlags("", location)
	if err != nil {
		log.Printf("INFO: cannot use kube config: %s\n", err)
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

// ApplyCRDs applies the CRDs to the cluster
func ApplyCRDs(kubeConfig *rest.Config) error {
	err := operator.ApplyCRDs(kubeConfig)
	if err != nil {
		return err
	}
	return nil
}

// NewEnvironment returns a new struct
func NewEnvironment(kubeConfig *rest.Config) *Environment {
	atomic.AddInt32(&namespaceCounter, 1)
	namespaceID := gomegaConfig.GinkgoConfig.ParallelNode*100 + int(namespaceCounter)

	return &Environment{
		ID:        namespaceID,
		Namespace: getNamespaceName(namespaceID),
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
		KubeConfig: kubeConfig,
	}
}

// SetupClientsets initializes kube clientsets
func (e *Environment) SetupClientsets() error {
	var err error
	e.Clientset, err = kubernetes.NewForConfig(e.KubeConfig)
	if err != nil {
		return err
	}

	e.VersionedClientset, err = versioned.NewForConfig(e.KubeConfig)
	if err != nil {
		return err
	}

	return nil
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
		return "", errors.Wrap(err, "getting the list of nodes")
	}

	if len(nodes.Items) == 0 {
		return "", fmt.Errorf("got an empty list of nodes")
	}

	addresses := nodes.Items[0].Status.Addresses
	if len(addresses) == 0 {
		return "", fmt.Errorf("node has an empty list of addresses")
	}

	return addresses[0].Address, nil
}

func getNamespaceName(namespaceCounter int) string {
	ns, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		ns = "default"
	}
	return ns + "-" + strconv.Itoa(int(namespaceCounter))
}
