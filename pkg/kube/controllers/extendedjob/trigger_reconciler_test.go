package extendedjob_test

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllersconfig"
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
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("TriggerReconciler", func() {
	Describe("Run", func() {
		var (
			env        testing.Catalog
			logs       *observer.ObservedLogs
			ctrsConfig *controllersconfig.ControllersConfig
			log        *zap.SugaredLogger
			mgr        *fakes.FakeManager
			query      *fakes.FakeQuery
			request    reconcile.Request
			reconciler reconcile.Reconciler

			runtimeObjects             []runtime.Object
			pod                        corev1.Pod
			jobList                    []v1alpha1.ExtendedJob
			setOwnerReferenceCallCount int
		)

		newRequest := func(pod corev1.Pod) reconcile.Request {
			return reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				},
			}
		}

		podGetStub := func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
			pod.DeepCopyInto(obj.(*corev1.Pod))
			return nil
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

		JustBeforeEach(func() {
			reconciler = NewTriggerReconciler(
				log,
				ctrsConfig,
				mgr,
				query,
				setOwnerReference,
			)
		})

		act := func() {
			reconciler.Reconcile(request)
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			logs, log = testing.NewTestLogger()
			ctrsConfig = &controllersconfig.ControllersConfig{ //Set the context to be TODO
				CtxTimeOut: 10 * time.Second,
				CtxType:    controllersconfig.NewContext(),
			}
			mgr = &fakes.FakeManager{}
			query = &fakes.FakeQuery{}
			setOwnerReferenceCallCount = 0
		})

		Context("when client fails", func() {
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				pod = env.DefaultPod("fake-pod")
				client.GetCalls(podGetStub)
				request = newRequest(pod)
			})

			Context("to get pod", func() {
				BeforeEach(func() {
					client.GetReturns(fmt.Errorf("fake-error"))
				})

				It("should log and return", func() {
					act()
					Expect(logs.FilterMessageSnippet("Failed to get the pod: fake-error").Len()).To(Equal(1))
					Expect(query.MatchCallCount()).To(Equal(0))
				})
			})

			Context("when client fails to list extended jobs", func() {
				BeforeEach(func() {
					client.ListReturns(fmt.Errorf("fake-error"))
				})

				It("should log list failure and return", func() {
					act()
					Expect(logs.FilterMessage("Failed to query extended jobs: fake-error").Len()).To(Equal(1))
					Expect(query.MatchCallCount()).To(Equal(0))
				})
			})

			Context("when client fails to create jobs", func() {
				BeforeEach(func() {
					jobList = []v1alpha1.ExtendedJob{
						*env.DefaultExtendedJob("foo"),
						*env.DefaultExtendedJob("bar"),
					}
					client.ListCalls(jobListStub)
					query.MatchReturns(true)
					query.MatchStateReturns(true)
					client.CreateReturns(fmt.Errorf("fake-error"))
				})

				It("should log create error and continue with next job", func() {
					act()
					Expect(client.CreateCallCount()).To(Equal(2))
					Expect(logs.FilterMessageSnippet("Failed to create job for 'foo' via pod fake-pod/deleted: fake-error").Len()).To(Equal(1))
					Expect(logs.FilterMessageSnippet("Failed to create job for 'bar' via pod fake-pod/deleted: fake-error").Len()).To(Equal(1))
				})
			})
		})

		Context("when no extended job is present", func() {
			var client crc.Client

			BeforeEach(func() {
				pod = env.DefaultPod("fake-pod")
				runtimeObjects = []runtime.Object{&pod}
				client = fake.NewFakeClient(runtimeObjects...)
				mgr.GetClientReturns(client)

				request = newRequest(pod)
			})

			It("should not create jobs", func() {
				act()
				Expect(query.MatchCallCount()).To(Equal(0))
				obj := &batchv1.JobList{}
				err := client.List(context.Background(), &crc.ListOptions{}, obj)
				Expect(err).ToNot(HaveOccurred())
				Expect(obj.Items).To(HaveLen(0))
			})
		})

		Context("when extended jobs are present", func() {
			var (
				client crc.Client
			)

			BeforeEach(func() {
				pod = env.DefaultPod("fake-pod")
				otherPod := env.DefaultPod("other-fake-pod")
				runtimeObjects = []runtime.Object{
					env.DefaultExtendedJob("foo"),
					env.LongRunningExtendedJob("bar"),
					&pod,
					&otherPod,
				}
				client = fake.NewFakeClient(runtimeObjects...)
				mgr.GetClientReturns(client)

				query.MatchReturns(true)
				query.MatchStateReturns(true)
				request = newRequest(pod)
			})

			Context("when pod matches label selector", func() {
				It("should create jobs", func() {
					act()

					obj := &batchv1.JobList{}
					err := client.List(context.Background(), &crc.ListOptions{}, obj)
					Expect(err).ToNot(HaveOccurred())

					jobs := obj.Items
					Expect(jobs).To(HaveLen(2))
					Expect(jobs[0].Name).To(ContainSubstring("job-foo-"))
					Expect(jobs[0].Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"sleep", "1"}))
					Expect(jobs[1].Name).To(ContainSubstring("job-bar-"))
					Expect(jobs[1].Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"sleep", "15"}))
					Expect(logs.FilterMessageSnippet("Created job for 'foo' via pod fake-pod/deleted").Len()).To(Equal(1))
					Expect(logs.FilterMessageSnippet("Created job for 'bar' via pod fake-pod/deleted").Len()).To(Equal(1))
				})
			})

			Context("when setting owner reference fails", func() {
				setOwnerReferenceFail := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
					return fmt.Errorf("fake-error")
				}

				It("should log and continue", func() {
					reconciler = NewTriggerReconciler(
						log,
						ctrsConfig,
						mgr,
						query,
						setOwnerReferenceFail,
					)
					act()
					Expect(logs.FilterMessageSnippet("Failed to set owner reference on job for 'foo' via pod fake-pod: fake-error").Len()).To(Equal(1))
					Expect(logs.FilterMessageSnippet("Created job for 'foo' via pod fake-pod/deleted").Len()).To(Equal(1))
				})
			})

		})

	})
})
