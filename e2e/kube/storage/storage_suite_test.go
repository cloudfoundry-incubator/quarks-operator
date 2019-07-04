package storage_kube_test

import (
	"testing"

	"code.cloudfoundry.org/cf-operator/e2e/kube/e2ehelper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/integration/environment"
)

func TestKubeStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Kube Storage Suite")
}

var (
	nsTeardown environment.TearDownFunc
	namespace  string
)

var _ = BeforeEach(func() {
	var err error
	nsTeardown, err = e2ehelper.SetUpEnvironment()
	Expect(err).ToNot(HaveOccurred())
})

var _ = AfterEach(func() {
	if nsTeardown != nil {
		nsTeardown()
	}
})
