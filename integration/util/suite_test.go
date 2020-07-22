package util_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"

	"code.cloudfoundry.org/quarks-operator/integration/environment"
	utils "code.cloudfoundry.org/quarks-utils/testing/integration"
)

func TestUtil(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Util Suite")
}

var (
	env              *environment.Environment
	namespacesToNuke []string
	kubeConfig       *rest.Config
)

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	kubeConfig, err = utils.KubeConfig()
	if err != nil {
		fmt.Printf("WARNING: failed to get kube config: %v\n", err)
	}

	// Ginkgo node 1 gets to setup the CRDs
	err = environment.ApplyCRDs(kubeConfig)
	if err != nil {
		fmt.Printf("WARNING: failed to apply CRDs: %v\n", err)
	}

	return []byte{}
}, func([]byte) {
	var err error
	kubeConfig, err = utils.KubeConfig()
	if err != nil {
		fmt.Printf("WARNING: failed to get kube config: %v\n", err)
	}
})

var _ = BeforeEach(func() {
	env = environment.NewEnvironment(kubeConfig)
	err := env.SetupClientsets()
	if err != nil {
		fmt.Printf("Integration setup failed. Creating clientsets in %s: %v\n", env.Namespace, err)
	}
	err = env.SetupNamespace()
	if err != nil {
		fmt.Printf("WARNING: failed to setup namespace %s: %v\n", env.Namespace, err)
	}
	namespacesToNuke = append(namespacesToNuke, env.Namespace)

	err = env.StartOperator()
	if err != nil {
		fmt.Printf("WARNING: failed to start operator: %v\n", err)
	}
})

var _ = AfterEach(func() {
	environment.Teardown(env, CurrentGinkgoTestDescription().Failed)
})

var _ = AfterSuite(func() {
	utils.NukeNamespaces(namespacesToNuke)
	environment.NukeWebhooks(namespacesToNuke)
})
