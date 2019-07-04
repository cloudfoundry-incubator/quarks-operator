package e2ehelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	"github.com/onsi/ginkgo/config"
	"github.com/pkg/errors"
)

const (
	cfOperatorRelease    = "cf-operator"
	installTimeOutInSecs = "600"
	helmCmd              = "helm"
	kubeCtlCmd           = "kubectl"
)

var (
	nsIndex   int
	namespace string
)

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
		_, err = RunBinary(helmCmd, "install", fmt.Sprintf("%s%s", dir, "/../../../helm/cf-operator"),
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
				return false, fmt.Errorf("missing CRDs in cluster, %s not found", crdName)
			}
		}

		return true, nil
	}
	return false, errors.New("Missing CRDs in cluster")
}

// RunHelmBinaryWithCustomErr executes a desire binary
func RunHelmBinaryWithCustomErr(binaryName string, args ...string) error {
	cmd := exec.Command(binaryName, args...)
	stdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return &CustomError{strings.Join(cmd.Args, " "), string(stdOutput), err}
	}
	return nil
}

// RunBinary executes a desire binary and returns the stdOutput
func RunBinary(binaryName string, args ...string) ([]byte, error) {
	cmd := exec.Command(binaryName, args...)
	stdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return stdOutput, errors.Wrapf(err, "%s cmd, failed with the following error: %s", cmd.Args, string(stdOutput))
	}
	return stdOutput, nil
}

// GetTestNamespace generates a namespace based on a env variable
func GetTestNamespace() (string, error) {
	prefix, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		prefix = "default"
	}
	namespace := prefix + "-" + strconv.Itoa(config.GinkgoConfig.ParallelNode) + "-" + strconv.Itoa(int(nsIndex))
	nsIndex++

	_, err := RunBinary(kubeCtlCmd, "create", "ns", namespace)
	if err != nil {
		return "", err
	}

	return namespace, nil
}

// DeleteOperatorResources removes left overs from the cf-operator chart,
// like the webhook server certificate secret and the mutatingwebhookconfiguration,
// in a specific namespace
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

// CustomError containing stdOuput of a binary execution
type CustomError struct {
	Msg    string
	StdOut string
	Err    error
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("%s:%v:%v", e.Msg, e.Err, e.StdOut)
}

// ClusterCrd defines a list of CRDs
type ClusterCrd struct {
	Items []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Metadata   struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

// ContainsElement verify if a CRD exist
func ContainsElement(list *ClusterCrd, element string) bool {
	for _, n := range list.Items {
		if n.Metadata.Name == element {
			return true
		}
	}
	return false
}
