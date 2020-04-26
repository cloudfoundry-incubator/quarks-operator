package environment

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	gomegaConfig "github.com/onsi/ginkgo/config"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" //from https://github.com/kubernetes/client-go/issues/345
	"k8s.io/client-go/rest"

	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/operator"
	"code.cloudfoundry.org/cf-operator/testing"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	utils "code.cloudfoundry.org/quarks-utils/testing/integration"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// Environment starts our operator and handles interaction with the k8s
// cluster used in the tests
type Environment struct {
	*utils.Environment
	Machine
	testing.Catalog
}

var (
	namespaceCounter int32
)

// NewEnvironment returns a new struct
func NewEnvironment(kubeConfig *rest.Config) *Environment {
	atomic.AddInt32(&namespaceCounter, 1)
	namespaceID := gomegaConfig.GinkgoConfig.ParallelNode*100 + int(namespaceCounter)

	return &Environment{
		Environment: &utils.Environment{
			ID:         namespaceID,
			Namespace:  utils.GetNamespaceName(namespaceID),
			KubeConfig: kubeConfig,
			Config: &config.Config{
				CtxTimeOut:           10 * time.Second,
				MeltdownDuration:     10 * time.Second,
				MeltdownRequeueAfter: 1 * time.Second,
				Fs:                   afero.NewOsFs(),
			},
		},
		Machine: Machine{
			Machine: machine.NewMachine(),
		},
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

// NodeIP returns a public IP of a node
func (e *Environment) NodeIP() (string, error) {
	if override, ok := os.LookupEnv("CF_OPERATOR_NODE_IP"); ok {
		// The user has specified a particular node IP to use; return that.
		return override, nil
	}

	nodes, err := e.Clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
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

// ApplyCRDs applies the CRDs to the cluster
func ApplyCRDs(kubeConfig *rest.Config) error {
	err := operator.ApplyCRDs(context.Background(), kubeConfig)
	if err != nil {
		return err
	}
	return nil
}
