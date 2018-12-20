package extendedstatefulset_test

import (
	"context"
	"time"

	exss "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	exssc "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReconcileExtendedStatefulSet", func() {
	var (
		trueValue bool

		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		log        *zap.SugaredLogger
	)

	BeforeEach(func() {
		trueValue = true

		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()
	})

	JustBeforeEach(func() {
		reconciler = exssc.NewReconciler(log, manager, controllerutil.SetControllerReference)
	})

	Describe("Reconcile", func() {
		var (
			client client.Client
		)

		Context("Provides CR", func() {
			BeforeEach(func() {
				client = fake.NewFakeClient(
					&exss.ExtendedStatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo",
							Namespace: "default",
						},
						Spec: exss.ExtendedStatefulSetSpec{},
					},
				)
				manager.GetClientReturns(client)
			})

			It("creates new StatefulSet and requeue the reconcile", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5 * time.Second,
				}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta1.StatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})

			It("updates existing stateful set", func() {
				ess := &exss.ExtendedStatefulSet{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion:         ess.APIVersion,
								Kind:               ess.Kind,
								Name:               ess.Name,
								UID:                ess.UID,
								Controller:         &trueValue,
								BlockOwnerDeletion: &trueValue,
							},
						},
						Annotations: map[string]string{
							exss.AnnotationStatefulSetSHA1: "",
							exss.AnnotationVersion:         "1",
						},
					},
				}
				err = client.Create(context.TODO(), ss)
				Expect(err).ToNot(HaveOccurred())

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5 * time.Second,
				}))

				ss = &v1beta1.StatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})
		})

		Context("doesn't provide CR", func() {
			var (
				client *cfakes.FakeClient
			)
			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				manager.GetClientReturns(client)
			})

			It("doesn't create new StatefulSet if ExtendedStatefulSet was not found", func() {
				client.GetReturns(errors.NewNotFound(schema.GroupResource{}, "not found is requeued"))

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("throws an error if get ExtendedStatefulSet returns error", func() {
				client.GetReturns(errors.NewServiceUnavailable("fake-error"))

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
