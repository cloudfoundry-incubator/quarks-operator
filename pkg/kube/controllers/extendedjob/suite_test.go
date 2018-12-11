package extendedjob_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

func TestExtendedJob(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ExtendedJob Suite")
}

var _ = BeforeSuite(func() {
	// setup logging for controller-runtime
	logf.SetLogger(logf.ZapLogger(true))
})
