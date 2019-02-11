package extendedjob_test

import (
	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Query", func() {

	var (
		client fakes.FakeClient
		env    testing.Catalog
		log    *zap.SugaredLogger
		query  *QueryImpl
	)

	BeforeEach(func() {
		client = fakes.FakeClient{}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()
		query = NewQuery(&client, log)
	})

	Describe("MatchState", func() {
		var (
			job      v1alpha1.ExtendedJob
			podState ejv1.PodState
		)

		act := func() bool {
			return query.MatchState(job, podState)
		}

		Context("when matching delete pod status", func() {
			BeforeEach(func() {
				job = *env.OnDeleteExtendedJob("foo")
				podState = ejv1.PodStateDeleted
			})
			It("should match deleted job", func() {
				m := act()
				Expect(m).To(BeTrue())
			})
		})

		Context("when matching running pod status", func() {
			BeforeEach(func() {
				job = *env.DefaultExtendedJob("foo")
				podState = ejv1.PodStateReady
			})
			It("should match", func() {
				m := act()
				Expect(m).To(BeTrue())
			})
		})

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

				It("returns false", func() {
					m := act()
					Expect(m).To(BeFalse())
				})
			})

			Context("when pod is deleted", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("", map[string]string{})
				})

				It("returns false", func() {
					m := act()
					Expect(m).To(BeFalse())
				})
			})
		})

		Context("when using trigger selector matchexpressions", func() {
			BeforeEach(func() {
				job = *env.MatchExpressionExtendedJob("foo")
			})

			Context("when pod matches", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("matching", map[string]string{"env": "production"})
				})

				It("returns true", func() {
					m := act()
					Expect(m).To(BeTrue())
				})
			})

			Context("when pod does not match", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("matching", map[string]string{"env": "dev"})
				})

				It("returns false", func() {
					m := act()
					Expect(m).To(BeFalse())
				})
			})
		})

		Context("when using both matchlabels and matchexpression", func() {
			BeforeEach(func() {
				job = *env.ComplexMatchExtendedJob("foo")
			})

			Context("and only matchLabels match", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("matching", map[string]string{"key": "value"})
				})

				It("returns false", func() {
					m := act()
					Expect(m).To(BeFalse())
				})
			})

			Context("and only matchExpressions match", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("matching", map[string]string{"env": "production"})
				})

				It("returns false", func() {
					m := act()
					Expect(m).To(BeFalse())
				})
			})

			Context("and neither matchLabels nor matchExpressions match", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("matching", map[string]string{"key": "doesntmatch", "env": "dev"})
				})

				It("returns false", func() {
					m := act()
					Expect(m).To(BeFalse())
				})
			})

			Context("and both matchLabels and matchExpressions match", func() {
				BeforeEach(func() {
					pod = env.LabeledPod("matching", map[string]string{"key": "value", "env": "production"})
				})

				It("returns true", func() {
					m := act()
					Expect(m).To(BeTrue())
				})
			})
		})
	})
})
