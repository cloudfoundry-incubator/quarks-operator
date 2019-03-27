package extendedjob_test

import (
	"time"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ = Describe("PodState", func() {
	Describe("InferPodState", func() {
		var (
			pod  corev1.Pod
			pods []corev1.Pod
		)

		act := func() ejv1.PodState {
			return InferPodState(pod)
		}

		Context("when pod is deleted", func() {
			BeforeEach(func() {
				now := metav1.NewTime(time.Now())
				pods = []corev1.Pod{
					corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							DeletionTimestamp: &now,
						},
						Status: corev1.PodStatus{Phase: "Running"},
					},
					corev1.Pod{
						Status: corev1.PodStatus{Phase: "Succeeded"},
					},
				}
			})
			It("should match deleted job", func() {
				for _, pod = range pods {
					s := act()
					Expect(s).To(Equal(ejv1.PodStateDeleted))
				}
			})
		})

		Context("when pod is running", func() {
			BeforeEach(func() {
				pod = corev1.Pod{
					Status: corev1.PodStatus{
						Phase: "Running",
						Conditions: []corev1.PodCondition{
							corev1.PodCondition{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
			})
			It("should match running job", func() {
				s := act()
				Expect(s).To(Equal(ejv1.PodStateReady))
			})
		})

		Context("when pod is created", func() {
			BeforeEach(func() {
				pod = corev1.Pod{
					Status: corev1.PodStatus{
						Phase: "Pending",
						Conditions: []corev1.PodCondition{
							corev1.PodCondition{
								Type:   corev1.PodScheduled,
								Status: corev1.ConditionTrue,
							},
							corev1.PodCondition{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
			})
			It("should match created job", func() {
				s := act()
				Expect(s).To(Equal(ejv1.PodStateCreated))
			})
		})

		Context("when pod is notready", func() {
			BeforeEach(func() {
				pod = corev1.Pod{
					Status: corev1.PodStatus{
						Phase: "Pending",
						Conditions: []corev1.PodCondition{
							corev1.PodCondition{
								Type:   corev1.PodScheduled,
								Status: corev1.ConditionTrue,
							},
						},
					},
				}
			})
			It("should match notready job", func() {
				s := act()
				Expect(s).To(Equal(ejv1.PodStateNotReady))
			})
		})
	})
})
