package extendedjob_test

import (
	"context"
	"fmt"

	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes/scheme"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ErrandReconciler", func() {
	Describe("Run", func() {
		var (
			env        testing.Catalog
			logs       *observer.ObservedLogs
			log        *zap.SugaredLogger
			mgr        *fakes.FakeManager
			request    reconcile.Request
			reconciler reconcile.Reconciler

			runtimeObjects             []runtime.Object
			exJob                      ejv1.ExtendedJob
			setOwnerReferenceCallCount int
		)

		newRequest := func(exJob ejv1.ExtendedJob) reconcile.Request {
			return reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      exJob.Name,
					Namespace: exJob.Namespace,
				},
			}
		}

		exjobGetStub := func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
			exJob.DeepCopyInto(obj.(*ejv1.ExtendedJob))
			return nil
		}

		setOwnerReference := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
			setOwnerReferenceCallCount++
			return nil
		}

		JustBeforeEach(func() {
			reconciler = NewErrandReconciler(
				log,
				mgr,
				setOwnerReference,
			)
		})

		act := func() (reconcile.Result, error) {
			return reconciler.Reconcile(request)
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			logs, log = testing.NewTestLogger()
			mgr = &fakes.FakeManager{}
			setOwnerReferenceCallCount = 0
		})

		Context("when client fails", func() {
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				exJob = env.ErrandExtendedJob("fake-exjob")
				client.GetCalls(exjobGetStub)
				request = newRequest(exJob)
			})

			Context("and the extended job does not exist", func() {
				BeforeEach(func() {
					client.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, "fake-error"))
				})

				It("should log and return, don't requeue", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(logs.FilterMessageSnippet("Failed to find extended job '/fake-exjob', not retrying:  \"fake-error\" not found").Len()).To(Equal(1))
				})
			})

			Context("to get the extended job", func() {
				BeforeEach(func() {
					client.GetReturns(fmt.Errorf("fake-error"))
				})

				It("should log and return, requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet("Failed to get the extended job '/fake-exjob': fake-error").Len()).To(Equal(1))
				})
			})

			Context("when client fails to update extended job", func() {
				BeforeEach(func() {
					client.UpdateReturns(fmt.Errorf("fake-error"))
				})

				It("should return and try to requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet("Failed to revert to 'Run=manually' on job 'fake-exjob': fake-error").Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(0))
					Expect(setOwnerReferenceCallCount).To(Equal(0))
				})
			})

			Context("when client fails to create jobs", func() {
				BeforeEach(func() {
					client.CreateReturns(fmt.Errorf("fake-error"))
				})

				It("should log create error and requeue", func() {
					_, err := act()
					Expect(logs.FilterMessageSnippet("Failed to create job 'fake-exjob': fake-error").Len()).To(Equal(1))
					Expect(err).To(HaveOccurred())
					Expect(client.CreateCallCount()).To(Equal(1))
					Expect(setOwnerReferenceCallCount).To(Equal(0))
				})
			})

			Context("when client fails to create jobs because it already exists", func() {
				BeforeEach(func() {
					client.CreateReturns(apierrors.NewAlreadyExists(schema.GroupResource{}, "fake-error"))
				})

				It("should log skip message and not requeue", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())

					Expect(logs.FilterMessageSnippet("Skip 'fake-exjob' triggered manually: already running").Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(1))
					Expect(setOwnerReferenceCallCount).To(Equal(0))
				})
			})

		})

		Context("when extended job is reconciled", func() {
			var (
				client crc.Client
			)

			BeforeEach(func() {
				exJob = env.ErrandExtendedJob("fake-pod")
				runtimeObjects = []runtime.Object{
					&exJob,
				}
				client = fake.NewFakeClient(runtimeObjects...)
				mgr.GetClientReturns(client)

				request = newRequest(exJob)
			})

			It("should set run back and create a job", func() {
				Expect(exJob.Spec.Run).To(Equal(ejv1.RunNow))

				result, err := act()
				Expect(err).ToNot(HaveOccurred())
				Expect(result.Requeue).To(BeFalse())

				obj := &batchv1.JobList{}
				err = client.List(context.TODO(), &crc.ListOptions{}, obj)
				Expect(err).ToNot(HaveOccurred())
				Expect(obj.Items).To(HaveLen(1))

				client.Get(
					context.TODO(),
					types.NamespacedName{
						Name:      exJob.Name,
						Namespace: exJob.Namespace,
					},
					&exJob,
				)
				Expect(exJob.Spec.Run).To(Equal(ejv1.RunManually))

			})
		})
	})
})
