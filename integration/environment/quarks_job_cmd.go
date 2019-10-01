package environment

import (
	"os/exec"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega/gexec"
)

// QuarksJobCmd helps to run the QuarksJob operator in tests
type QuarksJobCmd struct {
	Path string
}

// NewQuarksJobCmd returns the default QuarksJobCmd
func NewQuarksJobCmd() QuarksJobCmd {
	return QuarksJobCmd{}
}

// Build builds the quarks-job operator binary
func (q *QuarksJobCmd) Build() error {
	var err error
	q.Path, err = gexec.Build("code.cloudfoundry.org/quarks-job/cmd")
	return err
}

// Start starts the specified quarks-job in a namespace
func (q *QuarksJobCmd) Start(namespace string) error {
	cmd := exec.Command(q.Path,
		"-n", namespace,
		"-o", "cfcontainerization",
		"-r", "quarks-job",
		"-t", "v0.0.0-0.gb5dc240",
	)
	_, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	return err
}
