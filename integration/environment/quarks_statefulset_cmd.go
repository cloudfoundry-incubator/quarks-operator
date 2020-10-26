package environment

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
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
func (q *QuarksStatefulsetCmd) Start(id string, port int32) error {
	cmd := exec.Command(q.Path,
		"--operator-webhook-service-host", webhookHost(),
		"--operator-webhook-service-port", strconv.Itoa(int(port)),
		"--meltdown-duration", strconv.Itoa(defaultTestMeltdownDuration),
		"--meltdown-requeue-after", strconv.Itoa(defaultTestMeltdownRequeueAfter),
		"--monitored-id", id,
	)
	_, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	return err
}

func webhookHost() string {
	host, found := os.LookupEnv("CF_OPERATOR_WEBHOOK_SERVICE_HOST")
	if !found {
		host = "172.17.0.1"
	}
	return host
}
