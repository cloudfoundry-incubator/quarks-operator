package extendedjob_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	cclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejapi "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	ej "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedjob"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"
)

var _ = Describe("ReconcileExtendedJob", func() {
	var (
		manager      *cfakes.FakeManager
		reconciler   reconcile.Reconciler
		request      reconcile.Request
		log          *zap.SugaredLogger
		client       *cfakes.FakeClient
		podLogGetter *cfakes.FakePodLogGetter
		ejob         *ejapi.ExtendedJob
		job          *batchv1.Job
		pod          *corev1.Pod
		env          testing.Catalog
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object.(type) {
			case *ejapi.ExtendedJob:
				ejob.DeepCopyInto(object.(*ejapi.ExtendedJob))
				return nil
			case *batchv1.Job:
				job.DeepCopyInto(object.(*batchv1.Job))
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})
		client.ListCalls(func(context context.Context, options *cclient.ListOptions, object runtime.Object) error {
			list := corev1.PodList{
				Items: []corev1.Pod{*pod},
			}
			list.DeepCopyInto(object.(*corev1.PodList))
			return nil
		})
		manager.GetClientReturns(client)
		podLogGetter = &cfakes.FakePodLogGetter{}
		podLogGetter.GetReturns([]byte(`{"foo": "bar"}`), nil)
	})

	JustBeforeEach(func() {
		reconciler, _ = ej.NewJobReconciler(log, manager, podLogGetter)
		ejob, job, pod = env.DefaultExtendedJobWithSucceededJob("foo")
	})

	Context("With a succeeded Job", func() {
		Context("when output persistence is not configured", func() {
			It("does not persist output", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(0))
				Expect(reconcile.Result{}).To(Equal(result))
			})
		})

		Context("when output persistence is configured", func() {
			JustBeforeEach(func() {
				ejob.Spec.Output = ejapi.Output{
					NamePrefix:   "foo-",
					SecretLabels: map[string]string{"key": "value"},
				}
			})

			It("creates the secret and persists the output", func() {
				_, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(1))
			})

			It("adds configured labels to the generated secrets", func() {
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					secret := object.(*corev1.Secret)
					Expect(secret.ObjectMeta.Labels["key"]).To(Equal("value"))
					return nil
				})
				_, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(1))
			})
		})
	})

	Context("With a failed Job", func() {
		JustBeforeEach(func() {
			job.Status.Succeeded = 0
			job.Status.Failed = 1
		})

		Context("when WriteOnFailure is not set", func() {
			It("does not persist output", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(0))
				Expect(reconcile.Result{}).To(Equal(result))
			})
		})

		Context("when WriteOnFailure is not set", func() {
			JustBeforeEach(func() {
				ejob.Spec.Output = ejapi.Output{
					NamePrefix:     "foo-",
					WriteOnFailure: true,
				}
			})

			It("does persist the output", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(1))
				Expect(reconcile.Result{}).To(Equal(result))
			})
		})
	})
})
