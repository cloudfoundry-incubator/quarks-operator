package extendedstatefulset_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	exss "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	exssc "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileStatefulSetCleanup", func() {
	var (
		manager              *cfakes.FakeManager
		reconciler           reconcile.Reconciler
		request              reconcile.Request
		ctx                  context.Context
		log                  *zap.SugaredLogger
		config               *cfcfg.Config
		client               *cfakes.FakeClient
		desiredExStatefulSet *exss.ExtendedStatefulSet
		statefulSetV1        *v1beta2.StatefulSet
		statefulSetV2        *v1beta2.StatefulSet
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		// A desired ExtendedStatefulSet and its owned StatefulSets
		desiredExStatefulSet = &exss.ExtendedStatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
				UID:       "",
			},
			Spec: exss.ExtendedStatefulSetSpec{
				Template: v1beta2.StatefulSet{
					Spec: v1beta2.StatefulSetSpec{
						Replicas: pointers.Int32(1),
						Template: corev1.PodTemplateSpec{
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{
										Name: "fake-process",
									},
								},
							},
						},
					},
				},
			},
		}
		statefulSetV1 = &v1beta2.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo-v1",
				Namespace: "default",
				UID:       "",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:               "foo",
						UID:                "",
						Controller:         pointers.Bool(true),
						BlockOwnerDeletion: pointers.Bool(true),
					},
				},
				Annotations: map[string]string{
					exss.AnnotationVersion: "1",
				},
			},
			Status: v1beta2.StatefulSetStatus{
				CurrentRevision: "1",
			},
		}
		statefulSetV2 = &v1beta2.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo-v2",
				Namespace: "default",
				UID:       "",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:               "foo",
						UID:                "",
						Controller:         pointers.Bool(true),
						BlockOwnerDeletion: pointers.Bool(true),
					},
				},
				Annotations: map[string]string{
					exss.AnnotationVersion: "2",
				},
			},
			Status: v1beta2.StatefulSetStatus{
				CurrentRevision: "1",
			},
		}

		client = &cfakes.FakeClient{}
		manager.GetClientReturns(client)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo.bpm.fakepod-v1", Namespace: "default"}}
	})

	JustBeforeEach(func() {
		reconciler = exssc.NewStatefulSetCleanupReconciler(ctx, config, manager)
	})

	Context("when there is only one running version", func() {
		It("does not delete the only one version", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *exss.ExtendedStatefulSet:
					desiredExStatefulSet.DeepCopyInto(object)
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})
			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *v1beta2.StatefulSetList:
					list := v1beta2.StatefulSetList{
						Items: []v1beta2.StatefulSet{*statefulSetV1},
					}
					list.DeepCopyInto(object)
				}
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

		It("skips the reconcile when extendedStatefulSet is not found", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
		})

		It("handles an error when getting extendedStatefulSet ", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object.(type) {
				case *exss.ExtendedStatefulSet:
					return errors.New("some error")
				}

				return nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some error"))
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.ListCallCount()).To(Equal(0))
		})

		It("handles an error when listing statefulSets", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *exss.ExtendedStatefulSet:
					desiredExStatefulSet.DeepCopyInto(object)
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})
			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object.(type) {
				case *v1beta2.StatefulSetList:
					return errors.New("some error")
				}
				return nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some error"))
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.ListCallCount()).To(Equal(1))
		})
	})

	Context("when there is more than one version", func() {
		var (
			podV1 *corev1.Pod
			podV2 *corev1.Pod
		)

		BeforeEach(func() {
			podV1 = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-v1-0",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:               "foo-v1",
							UID:                "",
							Controller:         pointers.Bool(true),
							BlockOwnerDeletion: pointers.Bool(true),
						},
					},
					Labels: map[string]string{
						v1beta2.StatefulSetRevisionLabel: "1",
					},
				},
			}
			podV2 = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo-v2-0",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name:               "foo-v2",
							UID:                "",
							Controller:         pointers.Bool(true),
							BlockOwnerDeletion: pointers.Bool(true),
						},
					},
					Labels: map[string]string{
						v1beta2.StatefulSetRevisionLabel: "1",
					},
				},
			}
		})

		It("does not delete pods when latest version is not ready", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *exss.ExtendedStatefulSet:
					desiredExStatefulSet.DeepCopyInto(object)
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})
			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *v1beta2.StatefulSetList:
					list := v1beta2.StatefulSetList{
						Items: []v1beta2.StatefulSet{
							*statefulSetV1,
							*statefulSetV2,
						},
					}
					list.DeepCopyInto(object)
				case *corev1.PodList:
					list := corev1.PodList{
						Items: []corev1.Pod{
							*podV1,
							*podV2,
						},
					}
					list.DeepCopyInto(object)
				}
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

		It("deletes the old version when latest version is ready", func() {
			podV2.Status = corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			}
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *exss.ExtendedStatefulSet:
					desiredExStatefulSet.DeepCopyInto(object)
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})
			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *v1beta2.StatefulSetList:
					list := v1beta2.StatefulSetList{
						Items: []v1beta2.StatefulSet{
							*statefulSetV1,
							*statefulSetV2,
						},
					}
					list.DeepCopyInto(object)
				case *corev1.PodList:
					list := corev1.PodList{
						Items: []corev1.Pod{
							*podV1,
							*podV2,
						},
					}
					list.DeepCopyInto(object)
				}
				return nil
			})
			client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOption) error {
				switch object := object.(type) {
				case *v1beta2.StatefulSet:
					Expect(object.GetName()).To(Equal(fmt.Sprintf("%s-v%d", "foo", 1)))
					return nil
				}
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
			Expect(client.DeleteCallCount()).To(Equal(1))
		})

		It("handles an error when deleting a statefulSet", func() {
			podV2.Status = corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:   corev1.PodReady,
						Status: corev1.ConditionTrue,
					},
				},
			}
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *exss.ExtendedStatefulSet:
					desiredExStatefulSet.DeepCopyInto(object)
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			})
			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *v1beta2.StatefulSetList:
					list := v1beta2.StatefulSetList{
						Items: []v1beta2.StatefulSet{
							*statefulSetV1,
							*statefulSetV2,
						},
					}
					list.DeepCopyInto(object)
				case *corev1.PodList:
					list := corev1.PodList{
						Items: []corev1.Pod{
							*podV1,
							*podV2,
						},
					}
					list.DeepCopyInto(object)
				}
				return nil
			})
			client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOption) error {
				switch object.(type) {
				case *v1beta2.StatefulSet:
					return errors.New("some error")
				}
				return nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("some error"))
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.ListCallCount()).To(Equal(5))
			Expect(client.DeleteCallCount()).To(Equal(1))
		})
	})
})
