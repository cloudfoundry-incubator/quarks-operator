package extendedstatefulset_test

import (
	"context"
	"time"

	exsts "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReconcileExtendedStatefulSet", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		log        *zap.SugaredLogger
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()
	})

	JustBeforeEach(func() {
		reconciler = cfd.NewReconciler(log, manager, controllerutil.SetControllerReference)
	})

	Describe("Reconcile", func() {
		Context("when the manifest", func() {
			var (
				client *cfakes.FakeClient
			)
			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				client.GetStub = func(i context.Context, name types.NamespacedName, object runtime.Object) error {
					object = &exsts.ExtendedStatefulSet{
						TypeMeta: metav1.TypeMeta{
							Kind: "ExtendedStatefulSet",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo",
							Namespace: "default",
							UID:       "fake-uid",
						},
					}

					return nil
				}

				manager.GetClientReturns(client)
				manager.GetSchemeReturns(scheme.Scheme)
			})

			It("create new stateful set and requeue for extended stateful set", func() {
				reconciler.Reconcile(request)
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5 * time.Second,
				}))
			})
		})

	})
})
