package meltdown_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/meltdown"
)

var _ = Describe("Meltdown", func() {
	Describe("NewAnnotationWindow", func() {
		annotation := func(t time.Time) map[string]string {
			return map[string]string{
				meltdown.AnnotationLastReconcile: t.Format(time.RFC3339),
			}
		}

		It("returns true if we're in active meltdown", func() {
			start := time.Now()
			Expect(meltdown.NewAnnotationWindow(config.MeltdownDuration, annotation(start)).Contains(start)).To(BeTrue())
			now := start.Add(config.MeltdownDuration - 1*time.Second)
			Expect(meltdown.NewAnnotationWindow(config.MeltdownDuration, annotation(start)).Contains(now)).To(BeTrue())
		})

		It("returns false if we're outside the active meltdown", func() {
			start := time.Now()
			end := start.Add(config.MeltdownDuration)
			Expect(meltdown.NewAnnotationWindow(config.MeltdownDuration, annotation(start)).Contains(end)).To(BeFalse())

			before := start.Add(-1 * time.Second)
			Expect(meltdown.NewAnnotationWindow(config.MeltdownDuration, annotation(start)).Contains(before)).To(BeFalse())
		})
	})
})
