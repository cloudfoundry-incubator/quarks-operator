package extendedjob_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		log        *zap.SugaredLogger
		client     *cfakes.FakeClient
		ejob       *ejapi.ExtendedJob
		job        *batchv1.Job
		pod        *corev1.Pod
		env        testing.Catalog
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
			case *batchv1.Job:
				job.DeepCopyInto(object.(*batchv1.Job))
				//ase *corev1.Secret:
				//	return errors.NewNotFound(schema.GroupResource{}, "not found")
			}
			return nil
		})
		client.ListCalls(func(context context.Context, options *cclient.ListOptions, object runtime.Object) error {
			list := corev1.PodList{
				Items: []corev1.Pod{*pod},
			}
			list.DeepCopyInto(object.(*corev1.PodList))
			return nil
		})
		manager.GetClientReturns(client)
	})

	JustBeforeEach(func() {
		reconciler = ej.NewJobReconciler(log, manager)
	})

	Context("when output persistence is not configured", func() {
		BeforeEach(func() {
			ejob, job, pod = env.DefaultExtendedJobWithSucceededJob("foo")
			fmt.Println(ejob)
			fmt.Printf("%#v\n", job)
			fmt.Println(pod)
		})

		It("does not persist output", func() {
			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(reconcile.Result{}).To(Equal(result))
		})

		It("creates the secret and persists the output", func() {
			ejob.Spec.Output = ejapi.Output{
				SecretRef: "default/outputsecret",
			}
			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(1))
		})
	})
})
