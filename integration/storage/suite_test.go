package storage_test

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"k8s.io/client-go/rest"

	"code.cloudfoundry.org/quarks-operator/integration/environment"
	cmdHelper "code.cloudfoundry.org/quarks-utils/testing"
	utils "code.cloudfoundry.org/quarks-utils/testing/integration"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Storage Suite")
}

var (
	env              *environment.Environment
	namespacesToNuke []string
	kubeConfig       *rest.Config
	quarks           *environment.QuarksCmds
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

	// Ginkgo node 1 needs to build the quarks component binaries
	quarks = environment.NewQuarksCmds()
	err = quarks.Build()
	if err != nil {
		fmt.Printf("WARNING: %v\n", err)
	}

	data := quarks.Marshal()
	return data
}, func(data []byte) {
	var err error
	kubeConfig, err = utils.KubeConfig()
	if err != nil {
		fmt.Printf("WARNING: failed to get kube config: %v\n", err)
	}

	quarks = environment.NewQuarksCmds()
	err = quarks.Unmarshal(data)
	if err != nil {
		fmt.Printf("WARNING: failed to quarks binary paths from node 1: %v\n", err)
	}
})

var _ = BeforeEach(func() {
	env = environment.NewEnvironment(kubeConfig)

	SetDefaultEventuallyTimeout(env.PollTimeout)
	SetDefaultEventuallyPollingInterval(env.PollInterval)

	err := env.SetupClientsets()
	if err != nil {
		fmt.Printf("Integration setup failed. Creating clientsets for %s: %s\n", env.Namespace, err)
	}
	err = env.SetupNamespace()
	if err != nil {
		fmt.Printf("WARNING: failed to setup namespace %s: %v\n", env.Namespace, err)
	}
	namespacesToNuke = append(namespacesToNuke, env.Namespace)

	err = env.SetupQjobAccount()
	if err != nil {
		fmt.Printf("WARNING: failed to setup quarks-job operator service account: %s\n", err)
	}

	err = quarks.Job.Start(env.Config.MonitoredID)
	if err != nil {
		fmt.Printf("WARNING: failed to start quarks-job operator: %v\n", err)
	}

	err = quarks.Secret.Start(env.Config.MonitoredID)
	if err != nil {
		fmt.Printf("WARNING: failed to start quarks-secret operator: %v\n", err)
	}

	err = env.StartOperator()
	if err != nil {
		fmt.Printf("WARNING: failed to start operator: %v\n", err)
	}
})

var _ = AfterEach(func() {
	gexec.Kill()
	env.Teardown(CurrentGinkgoTestDescription().Failed)
})

var _ = AfterSuite(func() {
	// Nuking all namespaces at the end of the run
	for _, namespace := range namespacesToNuke {
		err := cmdHelper.DeleteNamespace(namespace)
		if err != nil && !env.NamespaceDeletionInProgress(err) {
			fmt.Printf("WARNING: failed to delete namespace %s: %v\n", namespace, err)
		}
		err = cmdHelper.DeleteWebhooks(namespace)
		if err != nil {
			fmt.Printf("WARNING: failed to delete mutatingwebhookconfiguration in %s: %v\n", namespace, err)
		}
	}
})
