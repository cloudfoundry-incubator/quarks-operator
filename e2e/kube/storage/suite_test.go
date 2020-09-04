package storage_kube_test

import (
	"fmt"
	"os"
	"testing"

	"code.cloudfoundry.org/quarks-utils/testing/e2ehelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestKubeStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Kube Storage Suite")
}

var (
	teardowns         []e2ehelper.TearDownFunc
	namespace         string
	operatorNamespace string
)

var _ = BeforeEach(func() {
	var teardown e2ehelper.TearDownFunc

	dir, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())

	chartPath := fmt.Sprintf("%s%s", dir, "/../../../helm/quarks-operator")

	namespace, operatorNamespace, teardown, err = e2ehelper.CreateNamespace()
	Expect(err).ToNot(HaveOccurred())
	teardowns = append(teardowns, teardown)

	teardown, err = e2ehelper.InstallChart(chartPath, operatorNamespace,
		"--set", fmt.Sprintf("global.singleNamespace.name=%s", namespace),
		"--set", fmt.Sprintf("global.monitoredID=%s", namespace),
		"--set", fmt.Sprintf("quarks-job.persistOutputClusterRole.name=%s", namespace),
	)
	Expect(err).ToNot(HaveOccurred())
	// prepend helm clean up
	teardowns = append([]e2ehelper.TearDownFunc{teardown}, teardowns...)
})

var _ = AfterEach(func() {
	e2ehelper.TearDownAll(teardowns)
})
