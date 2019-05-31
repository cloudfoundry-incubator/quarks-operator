package kube_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/cf-operator/integration/environment"
)

const (
	cfOperatorRelease    = "cf-operator"
	installTimeOutInSecs = "600"
	helmCmd              = "helm"
	kubeCtlCmd           = "kubectl"
)

var (
	cliPath      string
	stopOperator environment.StopFunc
	nsIndex      int
	nsTeardown   environment.TearDownFunc
	namespace    string
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
	nsIndex = 0

	RegisterFailHandler(FailAndCollectDebugInfo)
	RunSpecs(t, "E2E Kube Suite")
}

var _ = BeforeEach(func() {
	var err error
	nsTeardown, err = SetUpEnvironment()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	if nsTeardown != nil {
		nsTeardown()
	}
})

// SetUpEnvironment ensures helm binary can run,
// being able to reach tiller, and eventually it
// will install the cf-operator chart.
func SetUpEnvironment() (environment.TearDownFunc, error) {
	var crdExist bool

	// Ensure tiller is there, if not
	// then create it, via "init"
	err := RunHelmBinaryWithCustomErr(helmCmd, "version", "-s")
	if err != nil {
		switch err := err.(type) {
		case *CustomError:
			if strings.Contains(err.StdOut, "could not find tiller") {
				_, err := RunBinary(helmCmd, "init", "--wait")
				if err != nil {
					return nil, err
				}
			}
		default:
			return nil, err
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	crdExist, err = ClusterCrdsExist()
	if err != nil {
		return nil, err
	}

	namespace, err = GetTestNamespace()
	if err != nil {
		return nil, err
	}
	fmt.Println("Setting up test ns '" + namespace + "'...")

	if crdExist {
		_, err = RunBinary(helmCmd, "install", fmt.Sprintf("%s%s", dir, "/../../helm/cf-operator"),
			"--name", fmt.Sprintf("%s-%s", cfOperatorRelease, namespace),
			"--namespace", namespace,
			"--timeout", installTimeOutInSecs,
			"--set", "customResources.enableInstallation=false",
			"--wait")
		if err != nil {
			return nil, err
		}
	}

	teardownFunc := func() error {
		err = DeleteOperatorResources(namespace)
		if err != nil {
			return err
		}

		_, err := RunBinary(helmCmd, "delete", fmt.Sprintf("%s-%s", cfOperatorRelease, namespace), "--purge")
		if err != nil {
			return err
		}

		return nil
	}

	return teardownFunc, nil
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

func GetTestNamespace() (string, error) {
	prefix, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		prefix = "default"
	}
	namespace := prefix + "-" + strconv.Itoa(config.GinkgoConfig.ParallelNode) + "-" + strconv.Itoa(int(nsIndex))
	nsIndex += 1

	_, err := RunBinary(kubeCtlCmd, "create", "ns", namespace)
	if err != nil {
		return "", err
	}

	return namespace, nil
}

func DeleteOperatorResources(ns string) error {
	_, err := RunBinary(kubeCtlCmd, "--namespace", ns, "delete", "secret", "cf-operator-webhook-server-cert", "--ignore-not-found")
	if err != nil {
		return err
	}

	webHookName := fmt.Sprintf("%s-%s", "cf-operator-mutating-hook-", ns)
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
