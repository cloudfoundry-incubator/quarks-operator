package quarkslink

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("QuarksLink Annotations", func() {
	var (
		pod corev1.Pod
		env testing.Catalog
	)

	Describe("validEntanglement", func() {
		Context("when annotations are missing", func() {
			BeforeEach(func() {
				pod = env.AnnotatedPod("annotated", nil)
			})

			It("returns false", func() {
				Expect(validEntanglement(pod.GetAnnotations())).To(BeFalse())
			})
		})

		Context("when annotations are empty", func() {
			BeforeEach(func() {
				pod = env.AnnotatedPod("annotated", map[string]string{DeploymentKey: "", ConsumesKey: ""})
			})

			It("returns false", func() {
				Expect(validEntanglement(pod.GetAnnotations())).To(BeFalse())
			})
		})

		Context("when annotations are valid", func() {
			BeforeEach(func() {
				pod = env.AnnotatedPod("annotated", map[string]string{DeploymentKey: "foo", ConsumesKey: "nats"})
			})

			It("returns true", func() {
				Expect(validEntanglement(pod.GetAnnotations())).To(BeTrue())
			})
		})
	})
})
