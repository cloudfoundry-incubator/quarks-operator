package extendedjob_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("TriggerReconciler", func() {
	Describe("Run", func() {
		var (
			config     *cfcfg.Config
			ctx        context.Context
			env        testing.Catalog
			log        *zap.SugaredLogger
			logs       *observer.ObservedLogs
			mgr        *fakes.FakeManager
			query      *fakes.FakeQuery
			reconciler reconcile.Reconciler
			request    reconcile.Request

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

		jobListStub := func(ctx context.Context, obj runtime.Object, _ ...crc.ListOptionFunc) error {
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
				ctx,
				config,
				mgr,
				query,
				setOwnerReference,
				vss.NewVersionedSecretStore(mgr.GetClient()),
			)
		})

		act := func() (reconcile.Result, error) {
			return reconciler.Reconcile(request)
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)

			config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
			logs, log = helper.NewTestLogger()
			ctx = ctxlog.NewParentContext(log)
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
				pod.Status.Phase = "Running"
				pod.Status.Conditions = []corev1.PodCondition{
					{
						Type:   "Ready",
						Status: "True",
					},
				}
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
					Expect(logs.FilterMessageSnippet("Failed to create job for 'foo' via pod fake-pod/ready: fake-error").Len()).To(Equal(1))
					Expect(logs.FilterMessageSnippet("Failed to create job for 'bar' via pod fake-pod/ready: fake-error").Len()).To(Equal(1))
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
				err := client.List(ctx, obj)
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
				pod.Status.Phase = "Running"
				pod.Status.Conditions = []corev1.PodCondition{
					{
						Type:   "Ready",
						Status: "True",
					},
				}
				otherPod := env.DefaultPod("other-fake-pod")
				otherPod.Status.Phase = "Running"
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
					err := client.List(ctx, obj)
					Expect(err).ToNot(HaveOccurred())

					jobs := obj.Items
					Expect(jobs).To(HaveLen(2))
					Expect(jobs[0].Name).To(ContainSubstring("foo-fake-pod"))
					Expect(jobs[0].Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"sleep", "1"}))
					Expect(jobs[1].Name).To(ContainSubstring("bar-fake-pod"))
					Expect(jobs[1].Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"sleep", "15"}))
					Expect(logs.FilterMessageSnippet("Created job for 'foo' via pod fake-pod/ready").Len()).To(Equal(1))
					Expect(logs.FilterMessageSnippet("Created job for 'bar' via pod fake-pod/ready").Len()).To(Equal(1))
				})
			})

			Context("when setting owner reference fails", func() {
				setOwnerReferenceFail := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
					return fmt.Errorf("fake-error")
				}

				It("should log and continue", func() {
					reconciler = NewTriggerReconciler(
						ctx,
						config,
						mgr,
						query,
						setOwnerReferenceFail,
						vss.NewVersionedSecretStore(mgr.GetClient()),
					)

					_, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(logs.FilterMessageSnippet("failed to set owner reference on job for 'foo': fake-error").Len()).To(Equal(2))
					Expect(logs.FilterMessageSnippet("Created job for 'foo' via pod fake-pod/deleted").Len()).To(Equal(0))
				})
			})

		})

	})
})
