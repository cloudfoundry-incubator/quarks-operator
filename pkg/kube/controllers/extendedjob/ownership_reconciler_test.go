package extendedjob_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfctx "code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/finalizer"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"code.cloudfoundry.org/cf-operator/testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("OwnershipReconciler", func() {
	Describe("Reconcile", func() {
		var (
			env        testing.Catalog
			logs       *observer.ObservedLogs
			log        *zap.SugaredLogger
			config     *cfctx.Config
			mgr        *fakes.FakeManager
			request    reconcile.Request
			reconciler reconcile.Reconciler
			owner      *fakes.FakeOwner

			runtimeObjects             []runtime.Object
			extJob                     ejv1.ExtendedJob
			setOwnerReferenceCallCount int
		)

		newRequest := func(extJob ejv1.ExtendedJob) reconcile.Request {
			return reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      extJob.Name,
					Namespace: extJob.Namespace,
				},
			}
		}

		exjobGetStub := func(ctx context.Context, nn types.NamespacedName, obj runtime.Object) error {
			extJob.DeepCopyInto(obj.(*ejv1.ExtendedJob))
			return nil
		}

		setOwnerReference := func(owner, object metav1.Object, scheme *runtime.Scheme) error {
			setOwnerReferenceCallCount++
			return nil
		}

		JustBeforeEach(func() {
			reconciler = NewOwnershipReconciler(
				log,
				config,
				mgr,
				setOwnerReference,
				owner,
			)
		})

		act := func() (reconcile.Result, error) {
			return reconciler.Reconcile(request)
		}

		BeforeEach(func() {
			controllers.AddToScheme(scheme.Scheme)
			logs, log = helper.NewTestLogger()
			mgr = &fakes.FakeManager{}
			owner = &fakes.FakeOwner{}
			config = &cfctx.Config{
				CtxTimeOut: 1 * time.Second,
				CtxType:    cfctx.NewContext(),
			}
			setOwnerReferenceCallCount = 0
		})

		Context("when client fails to get the extended job", func() {
			var client fakes.FakeClient

			BeforeEach(func() {
				client = fakes.FakeClient{}
				mgr.GetClientReturns(&client)

				extJob = env.ErrandExtendedJob("fake-exjob")
				client.GetCalls(exjobGetStub)
				request = newRequest(extJob)
			})

			Context("because the extended job does not exist", func() {
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

			Context("to get the extended job for other reasons", func() {
				BeforeEach(func() {
					client.GetReturns(fmt.Errorf("fake-error"))
				})

				It("should log and return, requeue", func() {
					_, err := act()
					Expect(err).To(HaveOccurred())
					Expect(logs.FilterMessageSnippet("Failed to get the extended job '/fake-exjob': fake-error").Len()).To(Equal(1))
				})
			})
		})

		Context("when extended job exists", func() {
			var client crc.Client

			Context("when removing update on config change functionality", func() {
				BeforeEach(func() {
					extJob = env.ErrandExtendedJob("fake-pod")
					extJob.SetFinalizers([]string{finalizer.AnnotationFinalizer})
					extJob.Spec.UpdateOnConfigChange = false

					runtimeObjects = []runtime.Object{&extJob}
					client = fake.NewFakeClient(runtimeObjects...)
					mgr.GetClientReturns(client)

					request = newRequest(extJob)
				})

				It("removes owner references", func() {
					_, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(owner.RemoveOwnerReferencesCallCount()).To(Equal(1))
				})

				It("removes finalizer", func() {
					_, err := act()
					Expect(err).NotTo(HaveOccurred())
					ejob := &ejv1.ExtendedJob{}
					client.Get(context.Background(), request.NamespacedName, ejob)
					Expect(ejob.GetFinalizers()).To(BeEmpty())
				})

				It("returns and does not call Sync", func() {
					result, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(owner.SyncCallCount()).To(Equal(0))
					Expect(result).To(Equal(reconcile.Result{}))
				})
			})

			Context("when deleting extended job", func() {
				BeforeEach(func() {
					extJob = env.ErrandExtendedJob("fake-pod")
					extJob.SetFinalizers([]string{finalizer.AnnotationFinalizer})
					extJob.Spec.UpdateOnConfigChange = true
					now := metav1.NewTime(time.Now())
					extJob.SetDeletionTimestamp(&now)

					runtimeObjects = []runtime.Object{&extJob}
					client = fake.NewFakeClient(runtimeObjects...)
					mgr.GetClientReturns(client)

					request = newRequest(extJob)
				})

				It("removes owner references, finalizer and doesn't call Sync", func() {
					_, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(owner.RemoveOwnerReferencesCallCount()).To(Equal(1))
					Expect(owner.SyncCallCount()).To(Equal(0))
				})

				It("removes finalizer", func() {
					_, err := act()
					Expect(err).NotTo(HaveOccurred())
					ejob := &ejv1.ExtendedJob{}
					client.Get(context.Background(), request.NamespacedName, ejob)
					Expect(ejob.GetFinalizers()).To(BeEmpty())
				})
			})

			Context("when update on config change is enabled", func() {
				BeforeEach(func() {
					extJob = env.ErrandExtendedJob("fake-pod")
					extJob.Spec.UpdateOnConfigChange = true

					runtimeObjects = []runtime.Object{&extJob}
					client = fake.NewFakeClient(runtimeObjects...)
					mgr.GetClientReturns(client)

					request = newRequest(extJob)
				})

				It("calls sync", func() {
					_, err := act()
					Expect(err).NotTo(HaveOccurred())
					Expect(owner.SyncCallCount()).To(Equal(1))
					Expect(owner.RemoveOwnerReferencesCallCount()).To(Equal(0))
				})

				It("adds missing finalizer", func() {
					_, err := act()
					Expect(err).NotTo(HaveOccurred())
					ejob := &ejv1.ExtendedJob{}
					client.Get(context.Background(), request.NamespacedName, ejob)
					Expect(ejob.GetFinalizers()).NotTo(BeEmpty())
				})
			})
		})

	})
})
