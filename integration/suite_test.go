package integration_test

import (
	"fmt"
	"testing"
	"time"

	"code.cloudfoundry.org/cf-operator/integration/environment"
	operatortesting "code.cloudfoundry.org/cf-operator/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var (
	env                  *environment.Environment
	stopOperator         environment.StopFunc
	storageClassTeardown environment.TearDownFunc
	storageClassName     string
)

var _ = BeforeSuite(func() {
	env = environment.NewEnvironment()

	var err error
	stopOperator, err = env.Setup()
	Expect(err).NotTo(HaveOccurred())

	storageClassName = fmt.Sprintf("testsc-%s", operatortesting.RandString(5))
	storageClass := env.DefaultStorageClass(storageClassName)
	_, storageClassTeardown, err = env.CreateStorageClass(storageClass)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	defer func(tdf environment.TearDownFunc) { Expect(tdf()).To(Succeed()) }(storageClassTeardown)

	if stopOperator != nil {
		time.Sleep(3 * time.Second)
		defer stopOperator()
	}
})
