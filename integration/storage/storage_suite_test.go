package storage_test

import (
	"fmt"
	"testing"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Storage Suite")
}

var (
	env              *environment.Environment
	namespacesToNuke []string
	kubeCtlCmd       = "kubectl"
)

var _ = BeforeEach(func() {
	env = environment.SetupNamespace()
	namespacesToNuke = append(namespacesToNuke, env.Namespace)
})

var _ = AfterEach(func() {
	env.Teardown(CurrentGinkgoTestDescription().Failed)
})

var _ = AfterSuite(func() {
	// Nuking all namespaces at the end of the run
	for _, namespace := range namespacesToNuke {
		err := environment.DeleteNamespace(namespace, kubeCtlCmd)
		if err != nil {
			fmt.Printf("WARNING: failed to delete namespace %s: %v\n", namespace, err)
		}
	}
})
