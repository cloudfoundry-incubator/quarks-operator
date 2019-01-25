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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

			runtimeObjects             []runtime.Object
			jobList                    []v1alpha1.ExtendedJob
			setOwnerReferenceCallCount int
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

		setOwnerReference := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
			setOwnerReferenceCallCount++
			return nil
		}

		act := func() {
			// can't put New() into BeforeEach, mgr.GetClient would be nil
			r := NewRunner(
				log,
				mgr,
				query,
				setOwnerReference,
			)
			r.Run()
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			logs, log = testing.NewTestLogger()
			mgr = &fakes.FakeManager{}
			query = &fakes.FakeQuery{}
			setOwnerReferenceCallCount = 0
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
		})

		Context("when client fails to create jobs", func() {
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				jobList = []v1alpha1.ExtendedJob{
					*env.DefaultExtendedJob("foo"),
					*env.DefaultExtendedJob("bar"),
				}
				client.ListCalls(jobListStub)
				client.CreateReturns(fmt.Errorf("fake-error"))
				fooPod := env.DefaultPod("fake-pod")
				query.MatchReturns([]PodEvent{
					PodEvent{Pod: &fooPod},
					PodEvent{Pod: &fooPod},
				})
			})

			It("should log create error and continue with next event", func() {
				act()
				Expect(client.CreateCallCount()).To(Equal(4))
				Expect(logs.FilterMessageSnippet("foo: failed to create job for pod fake-pod: fake-error").Len()).To(Equal(2))
				Expect(logs.FilterMessageSnippet("bar: failed to create job for pod fake-pod: fake-error").Len()).To(Equal(2))
				Expect(setOwnerReferenceCallCount).To(Equal(0))
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

				fooPod := env.DefaultPod("mypod")
				query.MatchReturns([]PodEvent{
					PodEvent{Pod: &fooPod},
					PodEvent{Pod: &fooPod},
				})

				client.UpdateReturns(fmt.Errorf("fake-error"))
			})

			It("should log and continue with next pod event", func() {
				act()
				Expect(logs.FilterMessageSnippet("foo: failed to update job timestamp on pod mypod: fake-error").Len()).To(Equal(2))
			})
		})

		Context("when no extended job is present", func() {
			var client crc.Client

			BeforeEach(func() {
				runtimeObjects = []runtime.Object{}
				client = fake.NewFakeClient(runtimeObjects...)
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
			var (
				client    crc.Client
				fooPod    corev1.Pod
				now       time.Time
				timestamp string
			)

			BeforeEach(func() {
				now = time.Now()
				before25 := now.Add(-25 * time.Minute)
				timestamp = strconv.FormatInt(before25.Unix(), 10)

				fooPod = env.AnnotatedPod("fake-pod", map[string]string{
					"job-ts-foo": timestamp,
				})
				barPod := env.AnnotatedPod("barPod", map[string]string{
					"job-ts-bar": timestamp,
				})
				runtimeObjects = []runtime.Object{
					env.DefaultExtendedJob("foo"),
					env.LongRunningExtendedJob("bar"),
					&fooPod,
					&barPod,
				}
				client = fake.NewFakeClient(runtimeObjects...)
				mgr.GetClientReturns(client)

				query.MatchReturns([]PodEvent{
					PodEvent{Pod: &fooPod},
					PodEvent{Pod: &barPod},
				})
			})

			It("should create a job for each pod matched by a extendedjob", func() {
				act()

				jobs, err := listJobs(client)
				Expect(err).ToNot(HaveOccurred())
				Expect(jobs).To(HaveLen(2))
				Expect(jobs[0].Name).To(ContainSubstring("job-foo-"))
				Expect(jobs[0].Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"sleep", "1"}))
				Expect(jobs[1].Name).To(ContainSubstring("job-bar-"))
				Expect(jobs[1].Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"sleep", "15"}))
				Expect(logs.FilterMessageSnippet("foo: created job for pod barPod").Len()).To(Equal(1))
				Expect(logs.FilterMessageSnippet("bar: created job for pod fake-pod").Len()).To(Equal(1))
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

			Context("when setting owner reference fails", func() {
				setOwnerReferenceFail := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
					return fmt.Errorf("fake-error")
				}

				It("should log and try to update timestamp", func() {
					r := NewRunner(
						log,
						mgr,
						query,
						setOwnerReferenceFail,
					)
					r.Run()
					Expect(logs.FilterMessageSnippet("foo: failed to set reference on job for pod barPod: fake-error").Len()).To(Equal(1))
					Expect(fooPod.GetAnnotations()["job-ts-foo"]).NotTo(Equal(""))
				})
			})

			Context("when old events are present", func() {
				BeforeEach(func() {
					before10 := now.Add(-10 * time.Minute)
					before20 := now.Add(-20 * time.Minute)
					before30 := now.Add(-30 * time.Minute)
					query.MatchReturns([]PodEvent{
						PodEvent{
							Pod:   &fooPod,
							Event: env.DatedPodEvent(before10),
						},
						PodEvent{
							Pod:   &fooPod,
							Event: env.DatedPodEvent(before20),
						},
						PodEvent{
							Pod:   &fooPod,
							Event: env.DatedPodEvent(before30),
						},
					})
				})

				It("should ignore old events", func() {
					act()
					jobs, err := listJobs(client)
					Expect(err).ToNot(HaveOccurred())
					Expect(jobs).To(HaveLen(2))
					Expect(setOwnerReferenceCallCount).To(Equal(2))
					Expect(fooPod.GetAnnotations()).To(HaveLen(2))
					Expect(fooPod.GetAnnotations()["job-ts-foo"]).NotTo(Equal(timestamp))
				})
			})
		})

	})
})
