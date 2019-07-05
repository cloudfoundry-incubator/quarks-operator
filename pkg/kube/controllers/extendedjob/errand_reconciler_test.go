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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("ErrandReconciler", func() {
	Describe("Reconcile", func() {
		var (
			env        testing.Catalog
			logs       *observer.ObservedLogs
			log        *zap.SugaredLogger
			mgr        *fakes.FakeManager
			request    reconcile.Request
			reconciler reconcile.Reconciler

			runtimeObjects             []runtime.Object
			eJob                       ejv1.ExtendedJob
			setOwnerReferenceCallCount int
		)

		newRequest := func(eJob ejv1.ExtendedJob) reconcile.Request {
			return reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      eJob.Name,
					Namespace: eJob.Namespace,
				},
			}
		}

		ejobGetStub := func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
			eJob.DeepCopyInto(obj.(*ejv1.ExtendedJob))
			return nil
		}

		setOwnerReference := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
			setOwnerReferenceCallCount++
			return nil
		}

		JustBeforeEach(func() {
			ctx := ctxlog.NewParentContext(log)
			config := &config.Config{CtxTimeOut: 10 * time.Second}
			reconciler = NewErrandReconciler(
				ctx,
				config,
				mgr,
				setOwnerReference,
				vss.NewVersionedSecretStore(mgr.GetClient()),
			)
		})

		act := func() (reconcile.Result, error) {
			return reconciler.Reconcile(request)
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			mgr = &fakes.FakeManager{}
			setOwnerReferenceCallCount = 0
			logs, log = helper.NewTestLogger()
		})

		Context("when client fails", func() {
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				eJob = env.ErrandExtendedJob("fake-ejob")
				client.GetCalls(ejobGetStub)
				request = newRequest(eJob)
			})

			Context("and the extended job does not exist", func() {
				BeforeEach(func() {
					client.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, "fake-error"))
				})

				It("should log and return, don't requeue", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())
					Expect(logs.FilterMessageSnippet("Failed to find extended job '/fake-ejob', not retrying:  \"fake-error\" not found").Len()).To(Equal(1))
				})
			})

			Context("to get the extended job", func() {
				BeforeEach(func() {
					client.GetReturns(fmt.Errorf("fake-error"))
				})

				It("should log and return, requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet("Failed to get the extended job '/fake-ejob': fake-error").Len()).To(Equal(1))
				})
			})

			Context("when client fails to update extended job", func() {
				BeforeEach(func() {
					client.UpdateReturns(fmt.Errorf("fake-error"))
				})

				It("should return and try to requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet("Failed to revert to 'trigger.strategy=manual' on job 'fake-ejob': fake-error").Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(0))
				})
			})

			Context("when client fails to create jobs", func() {
				BeforeEach(func() {
					client.CreateReturns(fmt.Errorf("fake-error"))
				})

				It("should log create error and requeue", func() {
					_, err := act()
					Expect(logs.FilterMessageSnippet("Failed to create job 'fake-ejob': fake-error").Len()).To(Equal(1))
					Expect(err).To(HaveOccurred())
					Expect(client.CreateCallCount()).To(Equal(1))
				})
			})

			Context("when client fails to create jobs because it already exists", func() {
				BeforeEach(func() {
					client.UpdateReturns(nil)
					client.CreateReturns(apierrors.NewAlreadyExists(schema.GroupResource{}, "fake-error"))
				})

				It("should log skip message and not requeue", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())

					Expect(logs.FilterMessageSnippet("Skip 'fake-ejob' triggered manually: already running").Len()).To(Equal(1))
					Expect(client.CreateCallCount()).To(Equal(1))
				})
			})

		})

		Context("when extended job is reconciled", func() {
			var (
				client crc.Client
			)

			Context("and the errand is a manual errand", func() {
				BeforeEach(func() {
					eJob = env.ErrandExtendedJob("fake-pod")
					runtimeObjects = []runtime.Object{
						&eJob,
					}
					client = fake.NewFakeClient(runtimeObjects...)
					mgr.GetClientReturns(client)

					request = newRequest(eJob)
				})

				It("should set run back and create a job", func() {
					Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerNow))

					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())

					obj := &batchv1.JobList{}
					err = client.List(context.Background(), &crc.ListOptions{}, obj)
					Expect(err).ToNot(HaveOccurred())
					Expect(obj.Items).To(HaveLen(1))

					client.Get(
						context.Background(),
						types.NamespacedName{
							Name:      eJob.Name,
							Namespace: eJob.Namespace,
						},
						&eJob,
					)
					Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerManual))
				})
			})

			Context("and the errand is an auto-errand", func() {
				BeforeEach(func() {
					eJob = env.AutoErrandExtendedJob("fake-pod")
					runtimeObjects = []runtime.Object{
						&eJob,
					}
					client = fake.NewFakeClient(runtimeObjects...)
					mgr.GetClientReturns(client)

					request = newRequest(eJob)
				})

				It("should set the trigger strategy to done and immediately trigger the job", func() {
					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())

					obj := &batchv1.JobList{}
					err = client.List(context.Background(), &crc.ListOptions{}, obj)
					Expect(err).ToNot(HaveOccurred())
					Expect(obj.Items).To(HaveLen(1))

					client.Get(
						context.Background(),
						types.NamespacedName{
							Name:      eJob.Name,
							Namespace: eJob.Namespace,
						},
						&eJob,
					)
					Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerDone))

				})
			})

			Context("and the auto-errand is updated on config change", func() {
				BeforeEach(func() {
					eJob = env.AutoErrandExtendedJob("fake-pod")
					eJob.Spec.UpdateOnConfigChange = true
					eJob.Spec.Trigger.Strategy = ejv1.TriggerOnce
					runtimeObjects = []runtime.Object{&eJob}
					client = fake.NewFakeClient(runtimeObjects...)
					mgr.GetClientReturns(client)

					request = newRequest(eJob)
				})

				It("should trigger the job", func() {
					result, err := act()
					Expect(err).ToNot(HaveOccurred())
					Expect(result.Requeue).To(BeFalse())

					obj := &batchv1.JobList{}
					err = client.List(context.Background(), &crc.ListOptions{}, obj)
					Expect(err).ToNot(HaveOccurred())
					Expect(obj.Items).To(HaveLen(1))

					client.Get(
						context.Background(),
						types.NamespacedName{
							Name:      eJob.Name,
							Namespace: eJob.Namespace,
						},
						&eJob,
					)
					Expect(eJob.Spec.Trigger.Strategy).To(Equal(ejv1.TriggerDone))

				})
			})
		})
	})
})
