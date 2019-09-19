package extendedstatefulset_test

import (
	"context"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("ReconcileExtendedStatefulSet", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		ctx        context.Context
		log        *zap.SugaredLogger
		config     *cfcfg.Config
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
	})

	JustBeforeEach(func() {
		reconciler = exssc.NewReconciler(ctx, config, manager, controllerutil.SetControllerReference, vss.NewVersionedSecretStore(manager.GetClient()))
	})

	Describe("Reconcile", func() {
		var (
			client client.Client
		)

		Context("Provides a extendedStatefulSet definition", func() {
			var (
				existingAnnotation string
				existingLabel      string
				existingEnv        string
				existingValue      string

				desiredExtendedStatefulSet *exss.ExtendedStatefulSet
			)

			BeforeEach(func() {
				existingAnnotation = "existing_annotation"
				existingLabel = "existing_label"
				existingEnv = "existing_env"
				existingValue = "existing_value"

				desiredExtendedStatefulSet = &exss.ExtendedStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "",
					},
					Spec: exss.ExtendedStatefulSetSpec{
						Template: v1beta2.StatefulSet{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{existingAnnotation: existingValue},
								Labels:      map[string]string{existingLabel: existingValue},
							},
							Spec: v1beta2.StatefulSetSpec{
								Replicas: util.Int32(1),
								Template: corev1.PodTemplateSpec{
									ObjectMeta: metav1.ObjectMeta{
										Annotations: map[string]string{existingAnnotation: existingValue},
										Labels:      map[string]string{existingLabel: existingValue},
									},
									Spec: corev1.PodSpec{
										Containers: []corev1.Container{
											{
												Env: []corev1.EnvVar{
													{
														Name:  existingEnv,
														Value: existingValue,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}

				client = fake.NewFakeClient(
					desiredExtendedStatefulSet,
				)
				manager.GetClientReturns(client)
			})

			It("creates new statefulSet and continues to reconcile when new version is not available", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta2.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})

			Context("When zones has the values", func() {
				var (
					zones []string
				)

				BeforeEach(func() {
					zones = []string{"z1", "z2", "z3"}
					desiredExtendedStatefulSet.Spec.Zones = zones
				})

				When("When zoneNodeLabel has default value", func() {
					BeforeEach(func() {
						client = fake.NewFakeClient(
							desiredExtendedStatefulSet,
						)
						manager.GetClientReturns(client)
					})

					It("Creates new version and updates AZ info", func() {
						result, err := reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{}))

						ess := &exss.ExtendedStatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
						Expect(err).ToNot(HaveOccurred())

						ssZ0 := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z0-v1", Namespace: "default"}, ssZ0)
						Expect(err).ToNot(HaveOccurred())

						ssZ1 := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z1-v1", Namespace: "default"}, ssZ1)
						Expect(err).ToNot(HaveOccurred())

						ssZ2 := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z2-v1", Namespace: "default"}, ssZ2)
						Expect(err).ToNot(HaveOccurred())

						for idx, ss := range []*v1beta2.StatefulSet{ssZ0, ssZ1, ssZ2} {
							Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

							// Check statefulSet labels and annotations
							statefulSetLabels := ss.GetLabels()
							Expect(statefulSetLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(exss.LabelAZIndex, strconv.Itoa(idx)))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(exss.LabelAZName, zones[idx]))

							statefulSetAnnotations := ss.GetAnnotations()
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(exss.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

							// Check pod labels and annotations
							podLabels := ss.Spec.Template.GetLabels()
							Expect(podLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(podLabels).Should(HaveKeyWithValue(exss.LabelAZIndex, strconv.Itoa(idx)))
							Expect(podLabels).Should(HaveKeyWithValue(exss.LabelAZName, zones[idx]))

							podAnnotations := ss.Spec.Template.GetAnnotations()
							Expect(podAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(podAnnotations).Should(HaveKeyWithValue(exss.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

							// Check pod affinity
							podAffinity := ss.Spec.Template.Spec.Affinity
							Expect(podAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).Should(ContainElement(corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      exss.DefaultZoneNodeLabel,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{zones[idx]},
									},
								},
							}))

							// Check container envs
							containers := ss.Spec.Template.Spec.Containers
							for _, container := range containers {
								envs := container.Env
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  existingEnv,
									Value: existingValue,
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvKubeAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvBoshAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvCfOperatorAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvCfOperatorAzIndex,
									Value: strconv.Itoa(idx + 1),
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvReplicas,
									Value: "1",
								}))
							}
						}
					})
				})

				When("When zoneNodeLabel has been specified", func() {
					var (
						customizedNodeLabel string
					)
					BeforeEach(func() {
						customizedNodeLabel = "fake-zone-label"
						desiredExtendedStatefulSet.Spec.ZoneNodeLabel = customizedNodeLabel

						client = fake.NewFakeClient(
							desiredExtendedStatefulSet,
						)
						manager.GetClientReturns(client)
					})

					It("Creates new version and updates AZ info", func() {
						result, err := reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{}))

						ess := &exss.ExtendedStatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
						Expect(err).ToNot(HaveOccurred())

						ssZ0 := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z0-v1", Namespace: "default"}, ssZ0)
						Expect(err).ToNot(HaveOccurred())

						ssZ1 := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z1-v1", Namespace: "default"}, ssZ1)
						Expect(err).ToNot(HaveOccurred())

						ssZ2 := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z2-v1", Namespace: "default"}, ssZ2)
						Expect(err).ToNot(HaveOccurred())

						for idx, ss := range []*v1beta2.StatefulSet{ssZ0, ssZ1, ssZ2} {
							Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

							// Check statefulSet labels and annotations
							statefulSetLabels := ss.GetLabels()
							Expect(statefulSetLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(exss.LabelAZIndex, strconv.Itoa(idx)))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(exss.LabelAZName, zones[idx]))

							statefulSetAnnotations := ss.GetAnnotations()
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(exss.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

							// Check pod labels and annotations
							podLabels := ss.Spec.Template.GetLabels()
							Expect(podLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(podLabels).Should(HaveKeyWithValue(exss.LabelAZIndex, strconv.Itoa(idx)))
							Expect(podLabels).Should(HaveKeyWithValue(exss.LabelAZName, zones[idx]))

							podAnnotations := ss.Spec.Template.GetAnnotations()
							Expect(podAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(podAnnotations).Should(HaveKeyWithValue(exss.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

							// Check pod affinity
							podAffinity := ss.Spec.Template.Spec.Affinity
							Expect(podAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).Should(ContainElement(corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      customizedNodeLabel,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{zones[idx]},
									},
								},
							}))

							// Check container envs
							containers := ss.Spec.Template.Spec.Containers
							for _, container := range containers {
								envs := container.Env
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  existingEnv,
									Value: existingValue,
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvKubeAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvBoshAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvCfOperatorAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  exssc.EnvCfOperatorAzIndex,
									Value: strconv.Itoa(idx + 1),
								}))
							}
						}
					})
				})
			})
		})

		Context("doesn't provide any extendedStatefulSet definition", func() {
			var (
				client *cfakes.FakeClient
			)
			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				manager.GetClientReturns(client)
			})

			It("doesn't create new statefulset if extendedStatefulSet was not found", func() {
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

		Context("when there are two versions", func() {
			var (
				desiredExtendedStatefulSet *exss.ExtendedStatefulSet
				v1StatefulSet              *v1beta2.StatefulSet
				v2StatefulSet              *v1beta2.StatefulSet
			)

			BeforeEach(func() {
				desiredExtendedStatefulSet = &exss.ExtendedStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "foo-uid",
					},
					Spec: exss.ExtendedStatefulSetSpec{
						Template: v1beta2.StatefulSet{
							Spec: v1beta2.StatefulSetSpec{
								Replicas: util.Int32(1),
							},
						},
					},
				}
				v1StatefulSet = &v1beta2.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v1",
						Namespace: "default",
						UID:       "foo-v1-uid",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo",
								UID:                "foo-uid",
								Controller:         util.Bool(true),
								BlockOwnerDeletion: util.Bool(true),
							},
						},
						Annotations: map[string]string{
							exss.AnnotationVersion: "1",
						},
					},
				}
				v2StatefulSet = &v1beta2.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo-v2",
						Namespace: "default",
						UID:       "foo-v2-uid",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo",
								UID:                "foo-uid",
								Controller:         util.Bool(true),
								BlockOwnerDeletion: util.Bool(true),
							},
						},
						Annotations: map[string]string{
							exss.AnnotationVersion: "2",
						},
					},
				}

				client = fake.NewFakeClient(
					desiredExtendedStatefulSet,
					v1StatefulSet,
					v2StatefulSet,
				)
				manager.GetClientReturns(client)
			})

			It("creates version 3 is running", func() {
				ss := &v1beta2.StatefulSet{}
				err := client.Get(context.Background(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(errors.IsNotFound(err)).To(BeTrue())

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})
		})
	})
})
