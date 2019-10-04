package extendedjob_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejapi "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	ej "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("ReconcileExtendedJob", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		log        *zap.SugaredLogger
		client     *cfakes.FakeClient
		ejob       *ejapi.ExtendedJob
		job        *batchv1.Job
		pod1       *corev1.Pod
		env        testing.Catalog
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		_, log = helper.NewTestLogger()

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *ejapi.ExtendedJob:
				ejob.DeepCopyInto(object)
				return nil
			case *batchv1.Job:
				job.DeepCopyInto(object)
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})
		client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
			switch object := object.(type) {
			case *corev1.PodList:
				list := corev1.PodList{
					Items: []corev1.Pod{*pod1},
				}
				list.DeepCopyInto(object)
			case *corev1.SecretList:
				list := corev1.SecretList{}
				list.DeepCopyInto(object)
			}
			return nil
		})
		manager.GetClientReturns(client)
	})

	JustBeforeEach(func() {
		ctx := ctxlog.NewParentContext(log)
		config := &config.Config{CtxTimeOut: 10 * time.Second}
		reconciler, _ = ej.NewJobReconciler(ctx, config, manager)
		ejob, job, pod1 = env.DefaultExtendedJobWithSucceededJob("foo")
	})

	Context("With a succeeded Job", func() {
		It("deletes the job immediately", func() {
			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.DeleteCallCount()).To(Equal(1))
		})
	})

	Context("With a failed Job", func() {
		JustBeforeEach(func() {
			job.Status.Succeeded = 0
			job.Status.Failed = 1
		})

		It("does not delete the job immediately", func() {
			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

		Context("when WriteOnFailure is not set", func() {
			It("does not persist output", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(0))
				Expect(reconcile.Result{}).To(Equal(result))
			})
		})
	})
})
