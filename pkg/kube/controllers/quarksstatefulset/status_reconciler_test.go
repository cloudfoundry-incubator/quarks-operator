package quarksstatefulset_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qstsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	qstscontroller "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/quarksstatefulset"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileQuarksStatefulSet", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		ctx        context.Context
		log        *zap.SugaredLogger
		config     *cfcfg.Config
		client     *cfakes.FakeClient

		desiredQStatefulSet *qstsv1a1.QuarksStatefulSet
		sts                 *appsv1.StatefulSet
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *qstsv1a1.QuarksStatefulSet:
				desiredQStatefulSet.DeepCopyInto(object)
				return nil
			}
			return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
		})
		client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
			switch object := object.(type) {
			case *appsv1.StatefulSetList:
				list := appsv1.StatefulSetList{
					Items: []appsv1.StatefulSet{*sts},
				}
				list.DeepCopyInto(object)
			}
			return nil
		})
		manager.GetClientReturns(client)

		client.StatusCalls(func() crc.StatusWriter { return &cfakes.FakeStatusWriter{} })
	})

	JustBeforeEach(func() {
		reconciler = qstscontroller.NewQuarksStatefulSetStatusReconciler(ctx, config, manager)
		desiredQStatefulSet = &qstsv1a1.QuarksStatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
				UID:       "",
			},
			Spec: qstsv1a1.QuarksStatefulSetSpec{
				Template: appsv1.StatefulSet{
					Spec: appsv1.StatefulSetSpec{
						Replicas: pointers.Int32(1),
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{{
									Name: "test-container",
								}},
							},
						},
					},
				},
			},
		}
	})

	Context("Provides a quarksStatefulSet definition", func() {

		It("updates new statefulSet and continues to reconcile when new version is not available", func() {
			sts = &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
					Annotations: map[string]string{
						qstsv1a1.AnnotationVersion: "1",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:               "foo",
							Kind:               "QuarksStatefulSet",
							UID:                "",
							Controller:         pointers.Bool(true),
							BlockOwnerDeletion: pointers.Bool(true),
						},
					},
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointers.Int32(2),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Name: "test-container",
							}},
						},
					},
				},
				Status: appsv1.StatefulSetStatus{
					Replicas:      2,
					ReadyReplicas: 2,
				},
			}

			client.UpdateCalls(func(context context.Context, object runtime.Object, _ ...crc.UpdateOption) error {
				switch qSts := object.(type) {
				case *qstsv1a1.QuarksStatefulSet:
					Expect(qSts.Status.Ready).To(BeTrue())
				}
				return nil
			})
			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})
})
