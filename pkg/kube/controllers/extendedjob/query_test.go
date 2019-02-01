package extendedjob_test

import (
	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Query", func() {

	var (
		client fakes.FakeClient
		env    testing.Catalog
		query  *QueryImpl
	)

	BeforeEach(func() {
		client = fakes.FakeClient{}
		query = NewQuery(&client)
	})

	Describe("Match", func() {
		var (
			job v1alpha1.ExtendedJob
			pod corev1.Pod
		)

		act := func() bool {
			return query.Match(job, pod)
		}

		Context("when using trigger selector matchlabels", func() {
			BeforeEach(func() {
				job = *env.DefaultExtendedJob("foo")
			})

			Context("when pod matches", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("matching", map[string]string{"key": "value"})
				})

				It("returns true", func() {
					m := act()
					Expect(m).To(BeTrue())
				})
			})

			Context("when pod does not match", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("other", map[string]string{"other": "value"})
				})

				It("returns true", func() {
					m := act()
					Expect(m).To(BeFalse())
				})
			})
		})
	})
})
