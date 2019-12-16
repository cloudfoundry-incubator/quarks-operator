package storage_kube_test

import (
	"fmt"
	"log"
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
	nsTeardown        e2ehelper.TearDownFunc
	namespace         string
	operatorNamespace string
)

var _ = BeforeEach(func() {
	var err error

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	chartPath := fmt.Sprintf("%s%s", dir, "/../../../helm/cf-operator")
	namespace, operatorNamespace, nsTeardown, err = e2ehelper.SetUpEnvironment(chartPath)
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	if nsTeardown != nil {
		nsTeardown()
	}
})
