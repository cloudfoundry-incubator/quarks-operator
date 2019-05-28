package kube_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/cf-operator/integration/environment"
)

func FailAndCollectDebugInfo(description string, callerSkip ...int) {
	fmt.Println("Collecting debug information...")
	out, err := exec.Command("../../testing/dump_env.sh", namespace).CombinedOutput()
	if err != nil {
		fmt.Println("Failed to run the `dump_env.sh` script", err)
	}
	fmt.Println(string(out))

	Fail(description, callerSkip...)
}
func TestE2EKube(t *testing.T) {
	RegisterFailHandler(FailAndCollectDebugInfo)
	RunSpecs(t, "E2E Kube Suite")
}

const (
	cfOperatorRelease    = "cf-operator"
	installTimeOutInSecs = "600"
	helmCmd              = "helm"
	kubeCtlCmd           = "kubectl"
)

var (
	cliPath      string
	stopOperator environment.StopFunc
)

var _ = BeforeSuite(func() {
	err := SetUpEnvironment()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterSuite(func() {
	err := TearDownEnvironment()
	Expect(err).ToNot(HaveOccurred())
})

// TearDownEnvironment uninstall the cf-operator
// related helm chart
func TearDownEnvironment() error {
	_, err := RunBinary(helmCmd, "delete", fmt.Sprintf("%s-%s", cfOperatorRelease, GetTestNamespace()), "--purge")
	if err != nil {
		return err
	}

	err = DeleteOperatorResources()
	if err != nil {
		return err
	}

	return nil
}

// SetUpEnvironment ensures helm binary can run,
// being able to reach tiller, and eventually it
// will install the cf-operator chart.
func SetUpEnvironment() error {
	var crdExist bool

	err := DeleteOperatorResources()
	if err != nil {
		return err
	}

	// Ensure tiller is there, if not
	// then create it, via "init"
	err = RunHelmBinaryWithCustomErr(helmCmd, "version", "-s")
	if err != nil {
		switch err := err.(type) {
		case *CustomError:
			if strings.Contains(err.StdOut, "could not find tiller") {
				_, err := RunBinary(helmCmd, "init", "--wait")
				if err != nil {
					return err
				}
			}
		default:
			return err
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	crdExist, err = ClusterCrdsExist()
	if err != nil {
		return err
	}

	if crdExist {
		_, err = RunBinary(helmCmd, "install", fmt.Sprintf("%s%s", dir, "/../../helm/cf-operator"),
			"--name", fmt.Sprintf("%s-%s", cfOperatorRelease, GetTestNamespace()),
			"--namespace", GetTestNamespace(),
			"--timeout", installTimeOutInSecs,
			"--set", "customResources.enableInstallation=false",
			"--wait")
		if err != nil {
			return err
		}
	}
	return nil
}

func ClusterCrdsExist() (bool, error) {
	customResource := &ClusterCrd{}
	stdOutput, err := RunBinary(kubeCtlCmd, "get", "crds", "-o=json")
	if err != nil {
		return false, err
	}

	d := json.NewDecoder(bytes.NewReader(stdOutput))
	if err := d.Decode(customResource); err != nil {
		return false, err
	}
	crds := []string{"boshdeployments.fissile.cloudfoundry.org",
		"extendedjobs.fissile.cloudfoundry.org",
		"extendedsecrets.fissile.cloudfoundry.org",
		"extendedstatefulsets.fissile.cloudfoundry.org"}

	if len(customResource.Items) > 0 {
		for _, crdName := range crds {
			if !ContainsElement(customResource, crdName) {
				return false, fmt.Errorf("Missing CRDs in cluster, %s not found", crdName)
			}
		}

		return true, nil
	}
	return false, errors.New("Missing CRDs in cluster")
}

func RunHelmBinaryWithCustomErr(binaryName string, args ...string) error {
	cmd := exec.Command(binaryName, args...)
	stdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return &CustomError{strings.Join(cmd.Args, " "), string(stdOutput), err}
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

func GetTestNamespace() string {
	namespace, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		namespace = "default"
	}
	return namespace
}

func DeleteOperatorResources() error {
	_, err := RunBinary(kubeCtlCmd, "--namespace", GetTestNamespace(), "delete", "secret", "cf-operator-webhook-server-cert", "--ignore-not-found")
	if err != nil {
		return err
	}

	webHookName := fmt.Sprintf("%s-%s", "cf-operator-mutating-hook-", GetTestNamespace())
	_, err = RunBinary(kubeCtlCmd, "delete", "mutatingwebhookconfiguration", webHookName, "--ignore-not-found")
	if err != nil {
		return err
	}

	return nil
}

type CustomError struct {
	Msg    string
	StdOut string
	Err    error
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("%s:%v:%v", e.Msg, e.Err, e.StdOut)
}

type ClusterCrd struct {
	Items []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

func ContainsElement(list *ClusterCrd, element string) bool {
	for _, n := range list.Items {
		if n.Metadata.Name == element {
			return true
		}
	}
	return false
}
