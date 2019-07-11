package e2ehelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/onsi/ginkgo/config"
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
	nsIndex   int
	namespace string
)

// SetUpEnvironment ensures helm binary can run,
// being able to reach tiller, and eventually it
// will install the cf-operator chart.
func SetUpEnvironment(chartPath string) (string, environment.TearDownFunc, error) {
	var crdExist bool

	// Ensure tiller is there, if not
	// then create it, via "init"
	err := RunHelmBinaryWithCustomErr(helmCmd, "version", "-s")
	if err != nil {
		switch err := err.(type) {
		case *CustomError:
			if strings.Contains(err.StdOut, "could not find tiller") {
				_, err := environment.RunBinary(helmCmd, "init", "--wait")
				if err != nil {
					return "", nil, err
				}
			}
		default:
			return "", nil, err
		}
	}

	crdExist, err = ClusterCrdsExist()
	if err != nil {
		return "", nil, err
	}

	namespace, err = CreateTestNamespace()
	if err != nil {
		return "", nil, err
	}
	fmt.Println("Setting up in test namespace '" + namespace + "'...")

	if crdExist {
		// TODO: find relative path here
		_, err = environment.RunBinary(helmCmd, "install", chartPath,
			"--name", fmt.Sprintf("%s-%s", cfOperatorRelease, namespace),
			"--namespace", namespace,
			"--timeout", installTimeOutInSecs,
			"--set", "customResources.enableInstallation=false",
			"--wait")
		if err != nil {
			return "", nil, err
		}
	}

	teardownFunc := func() error {
		var messages string
		err = DeleteWebhookSecret(namespace)
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}

		err = environment.DeleteWebhook(namespace, kubeCtlCmd)
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}

		_, err := environment.RunBinary(helmCmd, "delete", fmt.Sprintf("%s-%s", cfOperatorRelease, namespace), "--purge")
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}

		err = environment.DeleteNamespace(namespace, kubeCtlCmd)
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}

		if messages != "" {
			fmt.Printf("Failures while cleaning up test environment for '%s':\n %v", namespace, messages)
			return errors.New(messages)
		}
		fmt.Println("Cleaned up test environment for '" + namespace + "'")
		return nil
	}

	return namespace, teardownFunc, nil
}

// ClusterCrdsExist verify if cf-operator CRDs are already in place
func ClusterCrdsExist() (bool, error) {
	customResource := &ClusterCrd{}
	stdOutput, err := environment.RunBinary(kubeCtlCmd, "get", "crds", "-o=json")
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

// CreateTestNamespace generates a namespace based on a env variable
func CreateTestNamespace() (string, error) {
	prefix, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		prefix = "default"
	}
	namespace := prefix + "-" + strconv.Itoa(config.GinkgoConfig.ParallelNode) + "-" + strconv.Itoa(int(nsIndex))
	nsIndex++

	_, err := environment.RunBinary(kubeCtlCmd, "create", "ns", namespace)
	if err != nil {
		return "", err
	}

	return namespace, nil
}

// DeleteWebhookSecret removes left overs from the cf-operator chart,
// like the webhook server certificate secret in a specific namespace
func DeleteWebhookSecret(ns string) error {
	_, err := environment.RunBinary(kubeCtlCmd, "--namespace", ns, "delete", "secret", "cf-operator-webhook-server-cert", "--ignore-not-found")

	return err
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
