package e2ehelper

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo/config"
	"github.com/pkg/errors"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	"code.cloudfoundry.org/cf-operator/testing"
)

const (
	installTimeOutInSecs = "600"
	e2eFailedMessage     = "e2e test setting up environment failed."
)

var (
	nsIndex   int
	namespace string
)

// SetUpEnvironment ensures helm binary can run,
// being able to reach tiller, and eventually it
// will install the cf-operator chart.
func SetUpEnvironment(chartPath string) (string, environment.TearDownFunc, error) {
	// Ensure tiller is there, if not
	// then create it, via "init"
	err := testing.RunHelmBinaryWithCustomErr("version", "-s")
	if err != nil {
		switch err := err.(type) {
		case *testing.CustomError:
			if strings.Contains(err.StdOut, "could not find tiller") {
				err := testing.RunHelmBinaryWithCustomErr("init", "--wait")
				if err != nil {
					return "", nil, errors.Wrapf(err, "%s Helm init --wait command failed.", e2eFailedMessage)
				}
			}
		default:
			return "", nil, errors.Wrapf(err, "%s Helm version -s command failed", e2eFailedMessage)
		}
	}

	namespace, err = CreateTestNamespace()
	if err != nil {
		return "", nil, errors.Wrapf(err, "%s Creating test namespace failed.", e2eFailedMessage)
	}
	fmt.Println("Setting up in test namespace '" + namespace + "'...")

	// TODO: find relative path here
	err = testing.RunHelmBinaryWithCustomErr("install", chartPath,
		"--name", fmt.Sprintf("%s-%s", testing.CFOperatorRelease, namespace),
		"--namespace", namespace,
		"--timeout", installTimeOutInSecs,
		"--wait")
	if err != nil {
		return "", nil, errors.Wrapf(err, "%s Helm install command failed.", e2eFailedMessage)
	}

	// Add sleep for workaround for CI timeouts
	time.Sleep(10* time.Second)

	teardownFunc := func() error {
		var messages string
		err = testing.DeleteSecret(namespace, "cf-operator-webhook-server-cert")
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}

		err = testing.DeleteWebhooks(namespace)
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}

		err := testing.RunHelmBinaryWithCustomErr("delete", fmt.Sprintf("%s-%s", testing.CFOperatorRelease, namespace), "--purge")
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}

		err = testing.DeleteNamespace(namespace)
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
	customResource, err := testing.GetCRDs()
	if err != nil {
		return false, errors.Wrapf(err, "%s", "Kubectl get crds command failed.")
	}

	crds := []string{"boshdeployments.fissile.cloudfoundry.org",
		"extendedjobs.fissile.cloudfoundry.org",
		"extendedsecrets.fissile.cloudfoundry.org",
		"extendedstatefulsets.fissile.cloudfoundry.org"}

	if len(customResource.Items) > 0 {
		for _, crdName := range crds {
			if !customResource.ContainsElement(crdName) {
				return false, fmt.Errorf("missing CRDs in cluster: %s not found", crdName)
			}
		}

		return true, nil
	}
	return false, errors.New("Missing CRDs in cluster")
}

// CreateTestNamespace generates a namespace based on a env variable
func CreateTestNamespace() (string, error) {
	prefix, found := os.LookupEnv("TEST_NAMESPACE")
	if !found {
		prefix = "default"
	}
	namespace := prefix + "-" + strconv.Itoa(config.GinkgoConfig.ParallelNode) + "-" + strconv.Itoa(int(nsIndex))
	nsIndex++

	err := testing.CreateNamespace(namespace)
	if err != nil {
		return "", err
	}

	return namespace, nil
}
