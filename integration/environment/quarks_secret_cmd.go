package environment

import (
	"os/exec"
	"strconv"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

// QuarksSecretCmd helps to run the QuarksSecret operator in tests
type QuarksSecretCmd struct {
	Path string
}

// Build builds the quarks-secret operator binary
func (q *QuarksSecretCmd) Build() error {
	var err error
	q.Path, err = gexec.Build("code.cloudfoundry.org/quarks-secret/cmd")
	return err
}

// Start starts the specified quarks-secret in a namespace
func (q *QuarksSecretCmd) Start(id string) error {
	cmd := exec.Command(q.Path,
		"--apply-crd=false",
		"--meltdown-duration", strconv.Itoa(defaultTestMeltdownDuration),
		"--meltdown-requeue-after", strconv.Itoa(defaultTestMeltdownRequeueAfter),
		"--monitored-id", id,
	)
	_, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	return err
}
