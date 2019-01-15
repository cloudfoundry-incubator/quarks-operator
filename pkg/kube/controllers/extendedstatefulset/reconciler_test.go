package extendedstatefulset_test

import (
	"context"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	exss "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	exssc "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"

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

		Context("Provides a extendedstatefulset definition", func() {
			var (
				desiredExtendedStatefulSet *exss.ExtendedStatefulSet
				v1StatefulSet              *v1beta1.StatefulSet
				v1Pod                      *corev1.Pod
				v2Pod                      *corev1.Pod
			)

			BeforeEach(func() {
				desiredExtendedStatefulSet = &exss.ExtendedStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "",
					},
					Spec: exss.ExtendedStatefulSetSpec{
						Template: v1beta1.StatefulSet{
							Spec: v1beta1.StatefulSetSpec{
								Replicas: helper.Int32(1),
							},
						},
					},
				}
				v1StatefulSet = &v1beta1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v1",
						Namespace: "default",
						UID:       "",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo",
								UID:                "",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
						Annotations: map[string]string{
							exss.AnnotationStatefulSetSHA1: "",
							exss.AnnotationVersion:         "1",
						},
					},
				}
				v1Pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v1-0",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo-v1",
								UID:                "",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
					},
				}
				v2Pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v2-0",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo-v2",
								UID:                "",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
					},
				}

				client = fake.NewFakeClient(
					desiredExtendedStatefulSet,
				)
				manager.GetClientReturns(client)
			})

			It("creates new statefulset and continues to reconcile when new version is not available", func() {
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

			It("updates existing statefulset", func() {
				ess := &exss.ExtendedStatefulSet{}
				err := client.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				// Provide statefulset and its pod from exStateful set
				ss := v1StatefulSet
				err = client.Create(context.TODO(), ss)
				Expect(err).ToNot(HaveOccurred())

				pod := v1Pod
				pod.Status = corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				}

				err = client.Create(context.TODO(), pod)
				Expect(err).ToNot(HaveOccurred())

				// First reconcile creates new version because template has been update
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ss = &v1beta1.StatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

				// Create pod from statefulset foo-v2
				pod = v2Pod
				pod.Status = corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				}
				err = client.Create(context.TODO(), pod)
				Expect(err).ToNot(HaveOccurred())

				// Second reconcile deletes old version because new version is already available
				result, err = reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})
		})

		Context("doesn't provide any extendedstatefulset definition", func() {
			var (
				client *cfakes.FakeClient
			)
			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				manager.GetClientReturns(client)
			})

			It("doesn't create new statefulset if extendedstatefulset was not found", func() {
				client.GetReturns(errors.NewNotFound(schema.GroupResource{}, "not found is requeued"))

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("throws an error if get extendedstatefulset returns error", func() {
				client.GetReturns(errors.NewServiceUnavailable("fake-error"))

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("when there are three versions", func() {
			var (
				currentSha string

				desiredExtendedStatefulSet *exss.ExtendedStatefulSet
				v1StatefulSet              *v1beta1.StatefulSet
				v2StatefulSet              *v1beta1.StatefulSet
				v3StatefulSet              *v1beta1.StatefulSet
				v1Pod                      *corev1.Pod
				v2Pod                      *corev1.Pod
				v3Pod                      *corev1.Pod
			)

			BeforeEach(func() {
				currentSha = "c32bf2e0a70aec0fdd745e321163e4c680662dad"
				desiredExtendedStatefulSet = &exss.ExtendedStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "foo-uid",
					},
					Spec: exss.ExtendedStatefulSetSpec{
						Template: v1beta1.StatefulSet{
							Spec: v1beta1.StatefulSetSpec{
								Replicas: helper.Int32(1),
							},
						},
					},
				}
				v1StatefulSet = &v1beta1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v1",
						Namespace: "default",
						UID:       "foo-v1-uid",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo",
								UID:                "foo-uid",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
						Annotations: map[string]string{
							exss.AnnotationStatefulSetSHA1: "",
							exss.AnnotationVersion:         "1",
						},
					},
				}
				v2StatefulSet = &v1beta1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v2",
						Namespace: "default",
						UID:       "foo-v2-uid",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo",
								UID:                "foo-uid",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
						Annotations: map[string]string{
							exss.AnnotationStatefulSetSHA1: "",
							exss.AnnotationVersion:         "2",
						},
					},
				}
				v3StatefulSet = &v1beta1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v3",
						Namespace: "default",
						UID:       "foo-v3-uid",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo",
								UID:                "foo-uid",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
						Annotations: map[string]string{
							exss.AnnotationStatefulSetSHA1: currentSha,
							exss.AnnotationVersion:         "3",
						},
					},
				}
				v1Pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v1-0",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo-v1",
								UID:                "foo-v1-uid",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				}
				v2Pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v2-0",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo-v2",
								UID:                "foo-v2-uid",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				}
				v3Pod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v3-0",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo-v3",
								UID:                "foo-v3-uid",
								Controller:         helper.Bool(true),
								BlockOwnerDeletion: helper.Bool(true),
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				}

				client = fake.NewFakeClient(
					desiredExtendedStatefulSet,
					v1StatefulSet,
					v1Pod,
					v2StatefulSet,
					v2Pod,
					v3StatefulSet,
					v3Pod,
				)
				manager.GetClientReturns(client)
			})

			It("cleans up old versions and stops reconcile when newer V3 is running", func() {
				v2Pod.Status = corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				}
				err := client.Update(context.TODO(), v2Pod)
				Expect(err).ToNot(HaveOccurred())

				v3Pod.Status = corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				}
				err = client.Update(context.TODO(), v3Pod)
				Expect(err).ToNot(HaveOccurred())

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta1.StatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})

			It("Only cleans up old V1 version and continues to reconcile when latest V3 is not running and V2 is running", func() {
				v2Pod.Status = corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				}
				err := client.Update(context.TODO(), v2Pod)
				Expect(err).ToNot(HaveOccurred())

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5000000000,
				}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta1.StatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})

			It("does nothing and continues to reconcile when V1, V2 and V3 are all not running", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5000000000,
				}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta1.StatefulSet{}
				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())
				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())
				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

				err = client.Get(context.TODO(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())
				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})
		})
	})
})
