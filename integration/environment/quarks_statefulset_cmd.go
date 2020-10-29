package environment

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
	"github.com/pkg/errors"

	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

// QuarksStatefulsetCmd helps to run the QuarksStatefulset operator in tests
type QuarksStatefulsetCmd struct {
	Path string
}

// Build builds the quarks-statefulset operator binary
func (q *QuarksStatefulsetCmd) Build() error {
	var err error
	q.Path, err = gexec.Build("code.cloudfoundry.org/quarks-statefulset/cmd")
	return err
}

// Start starts the specified quarks-statefulset in a namespace
func (q *QuarksStatefulsetCmd) Start(id string, ns string, port int32) error {
	cmd := exec.Command(q.Path,
		"--operator-webhook-service-host", webhookHost(),
		"--operator-webhook-service-port", strconv.Itoa(int(port)),
		"--quarks-statefulset-namespace", ns,
		"--apply-crd=false",
		"--meltdown-duration", strconv.Itoa(defaultTestMeltdownDuration),
		"--meltdown-requeue-after", strconv.Itoa(defaultTestMeltdownRequeueAfter),
		"--monitored-id", id,
	)
	// could pass in io writer - or just run with --debug
	_, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	if err != nil {
		return err
	}

	err = helper.WaitForPort(
		"127.0.0.1",
		strconv.Itoa(int(port)),
		1*time.Minute)
	if err != nil {
		return errors.Wrapf(err, "Integration setup failed. Waiting for port %d failed.", port)
	}
	return nil
}

func webhookHost() string {
	host, found := os.LookupEnv("CF_OPERATOR_WEBHOOK_SERVICE_HOST")
	if !found {
		host = "172.17.0.1"
	}
	return host
}
