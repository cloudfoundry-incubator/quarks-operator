package integration_test

import (
	"fmt"
	"os/exec"
	"testing"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
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
		err := DeleteNamespace(namespace)
		if err != nil {
			fmt.Printf("WARNING: failed to delete namespace %s: %v\n", namespace, err)
		}
	}
})

func DeleteNamespace(ns string) error {
	fmt.Printf("Cleaning up namespace %s \n", ns)

	_, err := RunBinary(kubeCtlCmd, "delete", "--ignore-not-found", "namespace", ns)
	if err != nil {
		return err
	}

	return nil
}

func RunBinary(binaryName string, args ...string) ([]byte, error) {
	cmd := exec.Command(binaryName, args...)
	stdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return stdOutput, errors.Wrapf(err, "%s cmd, failed with the following error: %s", cmd.Args, string(stdOutput))
	}
	return stdOutput, nil
}
