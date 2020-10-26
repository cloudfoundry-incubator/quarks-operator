package environment

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/operator"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/operatorimage"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

// StartOperator prepares and starts the quarks-operator
func (e *Environment) StartOperator() error {
	mgr, err := e.setupCFOperator()
	if err != nil {
		return errors.Wrapf(err, "Setting up CF Operator failed.")
	}

	e.StartManager(mgr)

	err = helper.WaitForPort(
		"127.0.0.1",
		strconv.Itoa(int(e.Config.WebhookServerPort)),
		1*time.Minute)
	if err != nil {
		return errors.Wrapf(err, "Integration setup failed. Waiting for port %d failed.", e.Config.WebhookServerPort)
	}

	return nil
}

// SetupPort of the webhook server
func (e *Environment) SetupPort() error {
	port, err := getWebhookServicePort(e.ID)
	if err != nil {
		return err
	}
	e.Config.WebhookServerPort = port
	return nil
}

// QStsPort returns the port for the quarks statefulset webhook service
func (e *Environment) QStsPort() int32 {
	return e.Config.WebhookServerPort + 1
}

func (e *Environment) setupCFOperator() (manager.Manager, error) {
	var err error
	whh, found := os.LookupEnv("CF_OPERATOR_WEBHOOK_SERVICE_HOST")
	if !found {
		return nil, errors.Errorf("Please set CF_OPERATOR_WEBHOOK_SERVICE_HOST to the host/ip the operator runs on and try again")
	}
	e.Config.WebhookServerHost = whh

	if err != nil {
		return nil, err
	}

	// Server needs `GatewayPorts yes` in sshd_config`
	sshUser, shouldForwardPort := os.LookupEnv("ssh_server_user")
	if shouldForwardPort {
		var remoteAddr string
		var ok bool
		if remoteAddr, ok = os.LookupEnv("ssh_server_listen_address"); !ok {
			remoteAddr = whh
		}

		cmd := exec.Command(
			"ssh", "-nNT", "-i", "/tmp/cf-operator-tunnel-identity", "-o",
			"UserKnownHostsFile=/dev/null", "-o", "StrictHostKeyChecking=no",
			"-R", fmt.Sprintf("%s:%[2]d:localhost:%[2]d", remoteAddr, e.Config.WebhookServerPort),
			"-R", fmt.Sprintf("%s:%[2]d:localhost:%[2]d", remoteAddr, e.QStsPort()),
			fmt.Sprintf("%s@%s", sshUser, whh))

		session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to setup the SSH tunnel to %s", remoteAddr)
		}
		gomega.Eventually(session.Err, "20s", "50ms").Should(gbytes.Say("Permanently added"))
	}

	dockerImageOrg, found := os.LookupEnv("DOCKER_IMAGE_ORG")
	if !found {
		dockerImageOrg = "cfcontainerization"
	}

	dockerImageRepo, found := os.LookupEnv("DOCKER_IMAGE_REPOSITORY")
	if !found {
		dockerImageRepo = "quarks-operator"
	}

	dockerImageTag, found := os.LookupEnv("DOCKER_IMAGE_TAG")
	if !found {
		return nil, errors.Errorf("required environment variable DOCKER_IMAGE_TAG not set")
	}

	err = operatorimage.SetupOperatorDockerImage(dockerImageOrg, dockerImageRepo, dockerImageTag, corev1.PullIfNotPresent)
	if err != nil {
		return nil, err
	}

	ctx := e.SetupLoggerContext("quarks-tests")

	mgr, err := operator.NewManager(ctx, e.Config, e.KubeConfig, manager.Options{
		MetricsBindAddress: "0",
		LeaderElection:     false,
		Port:               int(e.Config.WebhookServerPort),
		Host:               "0.0.0.0",
	})

	return mgr, err
}

func getWebhookServicePort(namespaceCounter int) (int32, error) {
	port := int64(40000)
	portString, found := os.LookupEnv("CF_OPERATOR_WEBHOOK_SERVICE_PORT")
	if found {
		var err error
		port, err = strconv.ParseInt(portString, 10, 32)
		if err != nil {
			return -1, errors.Wrapf(err, "Parsing CF_OPERATOR_WEBHOOK_SERVICE_PORT '%s' failed", portString)
		}
	}
	return int32(port) + int32(namespaceCounter), nil
}
