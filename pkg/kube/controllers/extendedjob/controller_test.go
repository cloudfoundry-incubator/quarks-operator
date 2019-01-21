package extendedjob_test

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var stopChan chan struct{}

func noWait(f func(), t time.Duration, c <-chan struct{}) {
	f()
	close(stopChan)
}

var _ = Describe("ExtendedJob", func() {
	var (
		mgr  *fakes.FakeManager
		logs *observer.ObservedLogs
		log  *zap.SugaredLogger
	)

	BeforeEach(func() {
		mgr = &fakes.FakeManager{}
		logs, log = testing.NewTestLogger()
	})

	Describe("Add", func() {
		It("adds our controller to the manager", func() {
			err := extendedjob.Add(log, mgr)
			Expect(err).ToNot(HaveOccurred())
			Expect(mgr.AddCallCount()).To(Equal(1))
		})
	})

	Describe("Start", func() {
		var (
			c      *extendedjob.Controller
			runner fakes.FakeRunner
		)

		BeforeEach(func() {
			runner = fakes.FakeRunner{}
			c = extendedjob.NewExtendedJobController(log, mgr, noWait, &runner)
			stopChan = make(chan struct{})
		})

		It("should log when waking up and run", func() {
			err := c.Start(stopChan)
			Expect(err).ToNot(HaveOccurred())
			Expect(logs.FilterMessage("ExtendedJob controller wakeup").Len()).To(Equal(1))
			Expect(runner.RunCallCount()).To(Equal(1))
		})

	})
})
