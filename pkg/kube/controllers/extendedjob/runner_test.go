package extendedjob_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/client-go/kubernetes/scheme"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Runner", func() {
	Describe("Run", func() {
		var (
			env   testing.Catalog
			logs  *observer.ObservedLogs
			log   *zap.SugaredLogger
			mgr   *fakes.FakeManager
			query *fakes.FakeQuery

			exJobs  []runtime.Object
			jobList []v1alpha1.ExtendedJob
		)

		listJobs := func(client crc.Client) ([]batchv1.Job, error) {
			obj := &batchv1.JobList{}
			err := client.List(context.TODO(), &crc.ListOptions{}, obj)
			return obj.Items, err
		}

		jobListStub := func(ctx context.Context, ops *crc.ListOptions, obj runtime.Object) error {
			if list, ok := obj.(*v1alpha1.ExtendedJobList); ok {
				list.Items = jobList
			}
			return nil
		}

		act := func() {
			// can't put New() into BeforeEach, mgr.GetClient would be nil
			r := NewRunner(
				log,
				mgr,
				query,
			)
			r.Run()
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			logs, log = testing.NewTestLogger()
			mgr = &fakes.FakeManager{}
			query = &fakes.FakeQuery{}
		})

		Context("when client fails to list extended jobs", func() {
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)
			})

			It("should log list failure", func() {
				client.ListReturns(fmt.Errorf("fake-error"))

				act()
				Expect(logs.FilterMessage("failed to query extended jobs: fake-error").Len()).To(Equal(1))
			})

			It("should log create error and continue", func() {
				jobList = []v1alpha1.ExtendedJob{
					*env.DefaultExtendedJob("foo"),
					*env.DefaultExtendedJob("bar"),
				}
				client.ListCalls(jobListStub)
				client.CreateReturns(fmt.Errorf("fake-error"))
				query.MatchReturns([]PodEvent{
					PodEvent{Pod: env.DefaultPod("foo")},
				})

				act()
				Expect(client.CreateCallCount()).To(Equal(2))
				Expect(logs.FilterMessageSnippet("failed to create job for foo: fake-error").Len()).To(Equal(1))
				Expect(logs.FilterMessageSnippet("failed to create job for bar: fake-error").Len()).To(Equal(1))
			})
		})

		Context("when updating stamp on pod fails", func() {
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				jobList = []v1alpha1.ExtendedJob{
					*env.DefaultExtendedJob("foo"),
				}
				client.ListCalls(jobListStub)

				query.MatchReturns([]PodEvent{
					PodEvent{Pod: env.DefaultPod("foo")},
				})

				client.UpdateReturns(fmt.Errorf("fake-error"))
			})

			It("should log and return", func() {
				act()
				Expect(logs.FilterMessageSnippet("failed to update job stamp on pod foo: fake-error").Len()).To(Equal(1))
			})
		})

		Context("when no extended job is present", func() {
			var client crc.Client

			BeforeEach(func() {
				exJobs = []runtime.Object{}
				client = fake.NewFakeClient(exJobs...)
				mgr.GetClientReturns(client)
			})

			It("should not query events or create jobs", func() {
				act()

				obj := &batchv1.JobList{}
				err := client.List(context.TODO(), &crc.ListOptions{}, obj)
				Expect(err).ToNot(HaveOccurred())
				Expect(obj.Items).To(HaveLen(0))
				Expect(query.RecentPodEventsCallCount()).To(Equal(0))
			})
		})

		Context("when extended jobs are present", func() {
			var client crc.Client

			BeforeEach(func() {
				exJobs = []runtime.Object{
					env.DefaultExtendedJob("foo"),
					env.DefaultExtendedJob("bar"),
				}
				client = fake.NewFakeClient(exJobs...)
				mgr.GetClientReturns(client)

				query.MatchReturns([]PodEvent{
					PodEvent{Pod: env.DefaultPod("foo")},
					PodEvent{Pod: env.DefaultPod("bar")},
				})
			})

			It("should create a job for each pod matched by a extendedjob", func() {
				act()

				jobs, err := listJobs(client)
				Expect(err).ToNot(HaveOccurred())
				Expect(jobs).To(HaveLen(4))
				Expect(jobs[0].Name).To(ContainSubstring("job-foo-"))
			})

			Context("when querying events fails", func() {
				BeforeEach(func() {
					query.RecentPodEventsReturns([]corev1.Event{}, fmt.Errorf("fake-error"))
				})

				It("should log failure and return", func() {
					act()
					Expect(logs.FilterMessageSnippet("failed to query pod related events: fake-error").Len()).To(Equal(1))
					jobs, err := listJobs(client)
					Expect(err).ToNot(HaveOccurred())
					Expect(jobs).To(HaveLen(0))
				})
			})

			Context("when getting pods fails", func() {
				BeforeEach(func() {
					events := []corev1.Event{
						corev1.Event{},
						corev1.Event{},
					}
					query.RecentPodEventsReturns(events, nil)
					query.FetchPodsReturns([]PodEvent{}, fmt.Errorf("fake-error"))
				})

				It("should log and return", func() {
					act()
					Expect(logs.FilterMessageSnippet("failed to get pods for events: fake-error").Len()).To(Equal(1))
					jobs, err := listJobs(client)
					Expect(err).ToNot(HaveOccurred())
					Expect(jobs).To(HaveLen(0))
				})
			})

		})

		Context("when processing events", func() {
			var client crc.Client

			BeforeEach(func() {
				exJobs = []runtime.Object{
					env.DefaultExtendedJob("foo"),
				}
				client = fake.NewFakeClient(exJobs...)
				mgr.GetClientReturns(client)

				now := time.Now()
				before10 := now.Add(time.Minute * -10)
				before20 := now.Add(time.Minute * -20)
				before25 := now.Add(time.Minute * -25)
				before30 := now.Add(time.Minute * -30)
				pod := env.AnnotatedPod("foo", map[string]string{
					"job-foo": strconv.FormatInt(before25.Unix(), 10),
				})
				query.MatchReturns([]PodEvent{
					PodEvent{
						Pod:   pod,
						Event: env.DatedPodEvent(before10),
					},
					PodEvent{
						Pod:   pod,
						Event: env.DatedPodEvent(before20),
					},
					PodEvent{
						Pod:   pod,
						Event: env.DatedPodEvent(before30),
					},
				})
			})

			It("should ignore old events and only run once per pod", func() {
				act()
				jobs, err := listJobs(client)
				Expect(err).ToNot(HaveOccurred())
				Expect(jobs).To(HaveLen(1))
			})
		})

	})
})
