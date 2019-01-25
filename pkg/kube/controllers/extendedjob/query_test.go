package extendedjob_test

import (
	"context"
	"fmt"
	"time"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

	Describe("RecentPodEvents", func() {

		act := func() ([]corev1.Event, error) {
			return query.RecentPodEvents()
		}

		Context("when events exist", func() {
			BeforeEach(func() {
				now := time.Now()
				items := []corev1.Event{
					env.DatedPodEvent(now.Add(Backlog - 1)),
					env.DatedPodEvent(now.Add(-10 * time.Minute)),
					env.DatedPodEvent(now.Add(-20 * time.Minute)),
				}
				listStub := func(ctx context.Context, ops *crc.ListOptions, obj runtime.Object) error {
					if list, ok := obj.(*corev1.EventList); ok {
						list.Items = items
					}
					return nil
				}
				client.ListCalls(listStub)
			})

			It("returns all pod related events with changes in specified timeframe", func() {
				events, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(client.ListCallCount()).To(Equal(1))
				_, options, _ := client.ListArgsForCall(0)
				Expect(options.FieldSelector.String()).To(Equal("involvedObject.kind=Pod"))
				Expect(events).To(HaveLen(2))
			})
		})

		It("returns error if client list fails", func() {
			client.ListReturns(fmt.Errorf("fake-error"))
			_, err := act()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("FetchPods", func() {

		var (
			events []corev1.Event
			pods   []corev1.Pod
		)

		findPod := func(pods []corev1.Pod, name string) (pod corev1.Pod, found bool) {
			for _, p := range pods {
				if p.Name == name {
					found = true
					pod = p
					return
				}
			}
			return
		}

		podGetStub := func(ctx context.Context, key types.NamespacedName, obj runtime.Object) error {
			if _, ok := obj.(*corev1.Pod); !ok {
				return nil
			}
			if pod, found := findPod(pods, key.Name); found {
				pod.DeepCopyInto(obj.(*corev1.Pod))
				return nil
			}
			return apierrors.NewNotFound(corev1.Resource("Pod"), key.Name)
		}

		act := func() ([]PodEvent, error) {
			return query.FetchPods(events)
		}

		Context("when events exist", func() {
			BeforeEach(func() {
				pods = []corev1.Pod{
					env.DefaultPod("one"),
					env.DefaultPod("two"),
					env.DefaultPod("unrelated"),
				}
				events = []corev1.Event{
					env.DefaultPodEvent("one"),
					env.DefaultPodEvent("one"),
					env.DefaultPodEvent("two"),
				}
				client.GetCalls(podGetStub)
			})

			It("returns all events with pointer to the pod", func() {
				podEvents, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(podEvents).To(HaveLen(3))
				Expect(podEvents[0].Pod.Name).To(Equal(pods[0].Name))
				Expect(podEvents[0].Event).To(Equal(events[0]))
				Expect(podEvents[1].Pod.Name).To(Equal(pods[0].Name))
				Expect(podEvents[1].Event).To(Equal(events[1]))
				Expect(podEvents[2].Pod.Name).To(Equal(pods[1].Name))
				Expect(podEvents[2].Event).To(Equal(events[2]))
				podEvents[0].Pod.Name = "test"
				Expect(podEvents[0].Pod).To(Equal(podEvents[1].Pod))
			})
		})

		Context("when events reference unknown pods", func() {
			BeforeEach(func() {
				pods = []corev1.Pod{
					env.DefaultPod("one"),
				}
				events = []corev1.Event{
					env.DefaultPodEvent("one"),
					env.DefaultPodEvent("missing"),
				}
				client.GetCalls(podGetStub)
			})

			It("returns existing pods", func() {
				pods, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(pods).To(HaveLen(1))
			})
		})

		It("returns error if get fails", func() {
			events = []corev1.Event{corev1.Event{}}
			client.GetReturns(fmt.Errorf("fake-error"))

			_, err := act()
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Match", func() {
		var (
			job  v1alpha1.ExtendedJob
			pods []PodEvent
		)

		act := func() []PodEvent {
			return query.Match(job, pods)
		}

		Context("when input list is empty", func() {
			BeforeEach(func() {
				job = *env.DefaultExtendedJob("foo")
				pods = []PodEvent{}
			})

			It("returns empty list", func() {
				filtered := act()
				Expect(filtered).To(HaveLen(0))
			})
		})

		Context("when given events with pods", func() {
			BeforeEach(func() {
				job = *env.DefaultExtendedJob("foo")
				matchingPod := env.LabeledPod("matching", map[string]string{"key": "value"})
				otherPod := env.LabeledPod("other", map[string]string{"other": "value"})
				anotherPod := env.LabeledPod("another", map[string]string{"key": "other"})
				manyLabelsPod := env.LabeledPod("many", map[string]string{"key": "value", "test": "true"})
				pods = []PodEvent{
					PodEvent{Pod: &matchingPod},
					PodEvent{Pod: &otherPod},
					PodEvent{Pod: &anotherPod},
					PodEvent{Pod: &manyLabelsPod},
				}
			})

			It("returns only tuples with matching labels", func() {
				filtered := act()
				Expect(filtered).To(HaveLen(2))
			})
		})
	})
})
