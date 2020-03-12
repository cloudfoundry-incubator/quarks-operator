package quarksstatefulset_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
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

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	qstscontroller "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/quarksstatefulset"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
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
		reconciler = qstscontroller.NewReconciler(ctx, config, manager, controllerutil.SetControllerReference, vss.NewVersionedSecretStore(manager.GetClient()))
	})

	Describe("Reconcile", func() {
		var (
			client client.Client
		)

		Context("Provides a quarksStatefulSet definition", func() {
			var (
				existingAnnotation string
				existingLabel      string
				existingEnv        string
				existingValue      string

				desiredQStatefulSet *qstsv1a1.QuarksStatefulSet
			)

			BeforeEach(func() {
				existingAnnotation = "existing_annotation"
				existingLabel = "existing_label"
				existingEnv = "existing_env"
				existingValue = "existing_value"

				desiredQStatefulSet = &qstsv1a1.QuarksStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "",
					},
					Spec: qstsv1a1.QuarksStatefulSetSpec{
						Template: appsv1.StatefulSet{
							ObjectMeta: metav1.ObjectMeta{
								Annotations: map[string]string{existingAnnotation: existingValue},
								Labels:      map[string]string{existingLabel: existingValue},
							},
							Spec: appsv1.StatefulSetSpec{
								Replicas: pointers.Int32(1),
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
					desiredQStatefulSet,
				)
				manager.GetClientReturns(client)
			})

			It("creates new statefulSet and continues to reconcile when new version is not available", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ess := &qstsv1a1.QuarksStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &appsv1.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})

			It("sets no RollingUpdate even if replica=1", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ss := &appsv1.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("with multiple replicas", func() {
				var ss *appsv1.StatefulSet
				BeforeEach(func() {
					desiredQStatefulSet.Spec.Template.Spec.Replicas = pointers.Int32(3)
					client = fake.NewFakeClient(
						desiredQStatefulSet,
					)
					manager.GetClientReturns(client)
				})

				JustBeforeEach(func() {
					result, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))

					ss = &appsv1.StatefulSet{}
					err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ss)
					Expect(err).ToNot(HaveOccurred())

				})

				It("sets annotation to start canary rollout", func() {
					Expect(ss.Annotations).To(HaveKeyWithValue("quarks.cloudfoundry.org/canary-rollout-enabled", "true"))
				})

				It("sets pod label for az index to 0 needed by service selector", func() {
					Expect(ss.Spec.Template.GetLabels()).To(HaveKeyWithValue("quarks.cloudfoundry.org/az-index", "0"))
				})

			})

			Context("When zones has the values", func() {
				var (
					zones []string
				)

				BeforeEach(func() {
					zones = []string{"z1", "z2", "z3"}
					desiredQStatefulSet.Spec.Zones = zones
				})

				When("zoneNodeLabel has default value", func() {
					BeforeEach(func() {
						client = fake.NewFakeClient(
							desiredQStatefulSet,
						)
						manager.GetClientReturns(client)
					})

					It("Creates new version and updates AZ info", func() {
						result, err := reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{}))

						ess := &qstsv1a1.QuarksStatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
						Expect(err).ToNot(HaveOccurred())

						ssZ0 := &appsv1.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z0", Namespace: "default"}, ssZ0)
						Expect(err).ToNot(HaveOccurred())

						ssZ1 := &appsv1.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z1", Namespace: "default"}, ssZ1)
						Expect(err).ToNot(HaveOccurred())

						ssZ2 := &appsv1.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z2", Namespace: "default"}, ssZ2)
						Expect(err).ToNot(HaveOccurred())

						for idx, ss := range []*appsv1.StatefulSet{ssZ0, ssZ1, ssZ2} {
							Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

							// Check statefulSet labels and annotations
							statefulSetLabels := ss.GetLabels()
							Expect(statefulSetLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZIndex, strconv.Itoa(idx)))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZName, zones[idx]))

							statefulSetAnnotations := ss.GetAnnotations()
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(qstsv1a1.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

							// Check pod labels and annotations
							podLabels := ss.Spec.Template.GetLabels()
							Expect(podLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(podLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZIndex, strconv.Itoa(idx)))
							Expect(podLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZName, zones[idx]))
							Expect(podLabels).Should(HaveKeyWithValue(qstsv1a1.LabelQStsName, fmt.Sprintf("%s-z%d", ess.Name, idx)))

							podAnnotations := ss.Spec.Template.GetAnnotations()
							Expect(podAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(podAnnotations).Should(HaveKeyWithValue(qstsv1a1.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

							// Check pod affinity
							podAffinity := ss.Spec.Template.Spec.Affinity
							Expect(podAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms).Should(ContainElement(corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      qstsv1a1.DefaultZoneNodeLabel,
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
									Name:  qstscontroller.EnvKubeAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  qstscontroller.EnvBoshAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  qstscontroller.EnvCfOperatorAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  qstscontroller.EnvCFOperatorAZIndex,
									Value: strconv.Itoa(idx + 1),
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  qstscontroller.EnvReplicas,
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
						desiredQStatefulSet.Spec.ZoneNodeLabel = customizedNodeLabel

						client = fake.NewFakeClient(
							desiredQStatefulSet,
						)
						manager.GetClientReturns(client)
					})

					It("Creates new version and updates AZ info", func() {
						result, err := reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{}))

						ess := &qstsv1a1.QuarksStatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
						Expect(err).ToNot(HaveOccurred())

						ssZ0 := &appsv1.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z0", Namespace: "default"}, ssZ0)
						Expect(err).ToNot(HaveOccurred())

						ssZ1 := &appsv1.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z1", Namespace: "default"}, ssZ1)
						Expect(err).ToNot(HaveOccurred())

						ssZ2 := &appsv1.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-z2", Namespace: "default"}, ssZ2)
						Expect(err).ToNot(HaveOccurred())

						for idx, ss := range []*appsv1.StatefulSet{ssZ0, ssZ1, ssZ2} {
							Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

							// Check statefulSet labels and annotations
							statefulSetLabels := ss.GetLabels()
							Expect(statefulSetLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZIndex, strconv.Itoa(idx)))
							Expect(statefulSetLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZName, zones[idx]))

							statefulSetAnnotations := ss.GetAnnotations()
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(statefulSetAnnotations).Should(HaveKeyWithValue(qstsv1a1.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

							// Check pod labels and annotations
							podLabels := ss.Spec.Template.GetLabels()
							Expect(podLabels).Should(HaveKeyWithValue(existingLabel, existingValue))
							Expect(podLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZIndex, strconv.Itoa(idx)))
							Expect(podLabels).Should(HaveKeyWithValue(qstsv1a1.LabelAZName, zones[idx]))
							Expect(podLabels).Should(HaveKeyWithValue(qstsv1a1.LabelQStsName, fmt.Sprintf("%s-z%d", ess.Name, idx)))

							podAnnotations := ss.Spec.Template.GetAnnotations()
							Expect(podAnnotations).Should(HaveKeyWithValue(existingAnnotation, existingValue))
							Expect(podAnnotations).Should(HaveKeyWithValue(qstsv1a1.AnnotationZones, "[\"z1\",\"z2\",\"z3\"]"))

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
									Name:  qstscontroller.EnvKubeAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  qstscontroller.EnvBoshAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  qstscontroller.EnvCfOperatorAz,
									Value: zones[idx],
								}))
								Expect(envs).Should(ContainElement(corev1.EnvVar{
									Name:  qstscontroller.EnvCFOperatorAZIndex,
									Value: strconv.Itoa(idx + 1),
								}))
							}
						}
					})
				})
			})
		})

		Context("doesn't provide any quarksStatefulSet definition", func() {
			var (
				client *cfakes.FakeClient
			)

			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				manager.GetClientReturns(client)
			})

			It("doesn't create new statefulSet if quarksStatefulSet was not found", func() {
				client.GetReturns(errors.NewNotFound(schema.GroupResource{}, "not found is requeued"))

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("throws an error if get quarksStatefulSet returns error", func() {
				client.GetReturns(errors.NewServiceUnavailable("fake-error"))

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("fake-error"))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("when there are two versions", func() {
			var (
				desiredQStatefulSet *qstsv1a1.QuarksStatefulSet
				v2StatefulSet       *appsv1.StatefulSet
			)

			BeforeEach(func() {
				desiredQStatefulSet = &qstsv1a1.QuarksStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "foo-uid",
					},
					Spec: qstsv1a1.QuarksStatefulSetSpec{
						Template: appsv1.StatefulSet{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"test": "2",
								},
							},
							Spec: appsv1.StatefulSetSpec{
								Replicas: pointers.Int32(1),
							},
						},
					},
				}
				v2StatefulSet = &appsv1.StatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "foo-v2-uid",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:               "foo",
								UID:                "foo-uid",
								Controller:         pointers.Bool(true),
								BlockOwnerDeletion: pointers.Bool(true),
							},
						},
						Annotations: map[string]string{
							qstsv1a1.AnnotationVersion: "2",
						},
					},
				}

				client = fake.NewFakeClient(
					desiredQStatefulSet,
					v2StatefulSet,
				)
				manager.GetClientReturns(client)
			})

			It("creates version 3", func() {
				ss := &appsv1.StatefulSet{}
				err := client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ss)
				Expect(err).NotTo(HaveOccurred())
				Expect(ss.GetAnnotations()).To(HaveKeyWithValue(qstsv1a1.AnnotationVersion, "2"))

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ess := &qstsv1a1.QuarksStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
				Expect(ss.GetAnnotations()).To(HaveKeyWithValue(qstsv1a1.AnnotationVersion, "3"))
			})
		})
	})
})
