package extendedstatefulset_test

import (
	"context"
	"strconv"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	exss "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	exssc "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedstatefulset"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfctx "code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/finalizer"
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
		ctrsConfig *cfctx.Config
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()

		ctrsConfig = &cfctx.Config{ //Set the context to be TODO
			CtxTimeOut: 10 * time.Second,
			CtxType:    cfctx.NewContext(),
		}
	})

	JustBeforeEach(func() {
		reconciler = exssc.NewReconciler(log, ctrsConfig, manager, controllerutil.SetControllerReference)
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
				v1StatefulSet              *v1beta2.StatefulSet
				v1Pod                      *corev1.Pod
				v2Pod                      *corev1.Pod
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
								Replicas: helper.Int32(1),
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
				v1StatefulSet = &v1beta2.StatefulSet{
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

			It("creates new statefulSet and continues to reconcile when new version is not available", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5 * time.Second,
				}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta2.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})

			It("updates existing statefulSet", func() {
				ess := &exss.ExtendedStatefulSet{}
				err := client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				// Provide statefulset and its pod from exStateful set
				ss := v1StatefulSet
				err = client.Create(context.Background(), ss)
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

				err = client.Create(context.Background(), pod)
				Expect(err).ToNot(HaveOccurred())

				// First reconcile creates new version because template has been update
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ss = &v1beta2.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
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
				err = client.Create(context.Background(), pod)
				Expect(err).ToNot(HaveOccurred())

				// Second reconcile deletes old version because new version is already available
				result, err = reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())
			})

			Context("When volumeclaimTemplates has values", func() {
				var (
					volumeClaimTemplates []corev1.PersistentVolumeClaim
				)

				BeforeEach(func() {
					volumeClaimTemplates = []corev1.PersistentVolumeClaim{ //TODO se catalog for this
						{
							ObjectMeta: metav1.ObjectMeta{
								Name: "pvc",
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{
									"ReadWriteOnce",
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										"Storage": resource.Quantity{
											Format: "5Gi",
										},
									},
								},
							},
						},
					}
					desiredExtendedStatefulSet.Spec.Template.Spec.VolumeClaimTemplates = volumeClaimTemplates

					client = fake.NewFakeClient(
						desiredExtendedStatefulSet,
					)
					manager.GetClientReturns(client)
				})

				It("Version should be added to the volume claim template name", func() {
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())

					ss := &v1beta2.StatefulSet{}
					err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
					Expect(err).ToNot(HaveOccurred())
					Expect(ss.Spec.VolumeClaimTemplates[0].Name).To(Equal("pvc-v1"))
				})
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
						Expect(result).To(Equal(reconcile.Result{
							Requeue:      true,
							RequeueAfter: 5 * time.Second,
						}))

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
									Value: strconv.Itoa(idx),
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
						Expect(result).To(Equal(reconcile.Result{
							Requeue:      true,
							RequeueAfter: 5 * time.Second,
						}))

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
									Value: strconv.Itoa(idx),
								}))
							}
						}
					})
				})
			})
		})

		Context("Provides a extendedStatefulSet containing ConfigMaps and Secrets references", func() {
			var (
				desiredExtendedStatefulSet *exss.ExtendedStatefulSet
				configMap1                 *corev1.ConfigMap
				configMap2                 *corev1.ConfigMap
				secret1                    *corev1.Secret
				secret2                    *corev1.Secret
			)

			BeforeEach(func() {
				desiredExtendedStatefulSet = &exss.ExtendedStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "",
					},
					Spec: exss.ExtendedStatefulSetSpec{
						Template: v1beta2.StatefulSet{
							Spec: v1beta2.StatefulSetSpec{
								Replicas: helper.Int32(1),
								Template: corev1.PodTemplateSpec{
									Spec: corev1.PodSpec{
										Volumes: []corev1.Volume{
											{
												Name: "secret1",
												VolumeSource: corev1.VolumeSource{
													Secret: &corev1.SecretVolumeSource{
														SecretName: "example1",
													},
												},
											},
											{
												Name: "configmap1",
												VolumeSource: corev1.VolumeSource{
													ConfigMap: &corev1.ConfigMapVolumeSource{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: "example1",
														},
													},
												},
											},
										},
										Containers: []corev1.Container{
											{
												Name:  "container1",
												Image: "container1",
												EnvFrom: []corev1.EnvFromSource{
													{
														ConfigMapRef: &corev1.ConfigMapEnvSource{
															LocalObjectReference: corev1.LocalObjectReference{
																Name: "example1",
															},
														},
													},
													{
														SecretRef: &corev1.SecretEnvSource{
															LocalObjectReference: corev1.LocalObjectReference{
																Name: "example1",
															},
														},
													},
												},
											},
											{
												Name:  "container2",
												Image: "container2",
												Env: []corev1.EnvVar{
													{
														ValueFrom: &corev1.EnvVarSource{
															ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
																LocalObjectReference: corev1.LocalObjectReference{
																	Name: "example2",
																},
															},
														},
													},
													{
														ValueFrom: &corev1.EnvVarSource{
															SecretKeyRef: &corev1.SecretKeySelector{
																LocalObjectReference: corev1.LocalObjectReference{
																	Name: "example2",
																},
															},
														},
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
				configMap1 = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example1",
						Namespace: "default",
					},
					Data: map[string]string{
						"key1": "example1:key1",
					},
				}
				configMap2 = &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example2",
						Namespace: "default",
					},
					Data: map[string]string{
						"key2": "example2:key2",
					},
				}
				secret1 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example1",
						Namespace: "default",
					},
					StringData: map[string]string{
						"key3": "example1:key2",
					},
				}
				secret2 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example2",
						Namespace: "default",
					},
					StringData: map[string]string{
						"key2": "example2:key2",
					},
				}

				client = fake.NewFakeClient(
					desiredExtendedStatefulSet,
					configMap1,
					configMap2,
					secret1,
					secret2,
				)
				manager.GetClientReturns(client)
			})

			Context("When UpdateOnEnvChange is false", func() {
				It("creates new statefulSet and has correct resources", func() {
					result, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{
						Requeue:      true,
						RequeueAfter: 5 * time.Second,
					}))

					ess := &exss.ExtendedStatefulSet{}
					err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
					Expect(err).ToNot(HaveOccurred())

					ss := &v1beta2.StatefulSet{}
					err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
					Expect(err).ToNot(HaveOccurred())

					Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

					By("Keeps current OwnerReferences of configurations", func() {
						err = client.Get(context.Background(), types.NamespacedName{Name: configMap1.Name, Namespace: "default"}, configMap1)
						Expect(err).ToNot(HaveOccurred())
						err = client.Get(context.Background(), types.NamespacedName{Name: configMap2.Name, Namespace: "default"}, configMap2)
						Expect(err).ToNot(HaveOccurred())
						err = client.Get(context.Background(), types.NamespacedName{Name: secret1.Name, Namespace: "default"}, secret1)
						Expect(err).ToNot(HaveOccurred())
						err = client.Get(context.Background(), types.NamespacedName{Name: secret2.Name, Namespace: "default"}, secret2)
						Expect(err).ToNot(HaveOccurred())

						ownerRef := metav1.OwnerReference{
							APIVersion:         "fissile.cloudfoundry.org/v1alpha1",
							Kind:               "ExtendedStatefulSet",
							Name:               ess.Name,
							UID:                ess.UID,
							Controller:         helper.Bool(false),
							BlockOwnerDeletion: helper.Bool(true),
						}

						for _, obj := range []apis.Object{configMap1, configMap2, secret1, secret2} {
							Expect(obj.GetOwnerReferences()).ShouldNot(ContainElement(ownerRef))
						}
					})
				})
			})

			Context("When UpdateOnEnvChange is set true", func() {
				BeforeEach(func() {
					desiredExtendedStatefulSet.Spec.UpdateOnEnvChange = true

					client = fake.NewFakeClient(
						desiredExtendedStatefulSet,
						configMap1,
						configMap2,
						secret1,
						secret2,
					)
					manager.GetClientReturns(client)
				})

				When("A ConfigMap reference is removed", func() {
					It("Creates new version and updates the config hash in the StatefulSet Annotations", func() {
						result, err := reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{
							Requeue:      true,
							RequeueAfter: 5 * time.Second,
						}))

						ess := &exss.ExtendedStatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
						Expect(err).ToNot(HaveOccurred())

						ss := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
						Expect(err).ToNot(HaveOccurred())

						Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

						originalSHA1 := ss.Spec.Template.GetAnnotations()[exss.AnnotationConfigSHA1]

						// Remove "container2" which references Secret example2 and ConfigMap
						containers := ss.Spec.Template.Spec.Containers
						ess.Spec.Template.Spec.Template.Spec.Containers = []corev1.Container{containers[0]}
						err = client.Update(context.Background(), ess)
						Expect(err).ToNot(HaveOccurred())
						result, err = reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{
							Requeue:      true,
							RequeueAfter: 5 * time.Second,
						}))

						ss = &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
						Expect(err).ToNot(HaveOccurred())

						currentSHA1 := ss.Spec.Template.GetAnnotations()[exss.AnnotationConfigSHA1]

						By("Adds the OwnerReference from the orphaned configurations", func() {
							err = client.Get(context.Background(), types.NamespacedName{Name: configMap1.Name, Namespace: "default"}, configMap1)
							Expect(err).ToNot(HaveOccurred())
							err = client.Get(context.Background(), types.NamespacedName{Name: secret1.Name, Namespace: "default"}, secret1)
							Expect(err).ToNot(HaveOccurred())

							ownerRef := metav1.OwnerReference{
								APIVersion:         "fissile.cloudfoundry.org/v1alpha1",
								Kind:               "ExtendedStatefulSet",
								Name:               ess.Name,
								UID:                ess.UID,
								Controller:         helper.Bool(false),
								BlockOwnerDeletion: helper.Bool(true),
							}

							for _, obj := range []apis.Object{configMap1, secret1} {
								Expect(obj.GetOwnerReferences()).Should(ContainElement(ownerRef))
							}
						})

						By("Updates the config hash in the StatefulSet Annotations", func() {
							Expect(currentSHA1).ShouldNot(Equal(originalSHA1))
						})

						By("Adds a finalizer to the ExtendedStatefulSet", func() {
							err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
							Expect(err).ToNot(HaveOccurred())
							Expect(ess.GetFinalizers()).Should(ContainElement(finalizer.AnnotationFinalizer))
						})
					})
				})

				When("A ConfigMap reference is updated", func() {
					It("Preserves current version and updates the config hash in the StatefulSet Annotations", func() {
						result, err := reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{
							Requeue:      true,
							RequeueAfter: 5 * time.Second,
						}))

						ess := &exss.ExtendedStatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
						Expect(err).ToNot(HaveOccurred())

						ss := &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
						Expect(err).ToNot(HaveOccurred())
						Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

						originalSHA1 := ss.Spec.Template.GetAnnotations()[exss.AnnotationConfigSHA1]

						// Update "configMap1"
						err = client.Get(context.Background(), types.NamespacedName{Name: configMap1.Name, Namespace: "default"}, configMap1)
						Expect(err).ToNot(HaveOccurred())

						configMap1.Data["key1"] = "modified"
						err = client.Update(context.Background(), configMap1)
						Expect(err).ToNot(HaveOccurred())
						result, err = reconciler.Reconcile(request)
						Expect(err).ToNot(HaveOccurred())
						Expect(result).To(Equal(reconcile.Result{
							Requeue:      true,
							RequeueAfter: 5 * time.Second,
						}))

						ss = &v1beta2.StatefulSet{}
						err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
						Expect(err).ToNot(HaveOccurred())

						currentSHA1 := ss.Spec.Template.GetAnnotations()[exss.AnnotationConfigSHA1]

						By("Adds OwnerReferences to all configurations", func() {
							err = client.Get(context.Background(), types.NamespacedName{Name: configMap1.Name, Namespace: "default"}, configMap1)
							Expect(err).ToNot(HaveOccurred())
							err = client.Get(context.Background(), types.NamespacedName{Name: configMap2.Name, Namespace: "default"}, configMap2)
							Expect(err).ToNot(HaveOccurred())
							err = client.Get(context.Background(), types.NamespacedName{Name: secret1.Name, Namespace: "default"}, secret1)
							Expect(err).ToNot(HaveOccurred())
							err = client.Get(context.Background(), types.NamespacedName{Name: secret2.Name, Namespace: "default"}, secret2)
							Expect(err).ToNot(HaveOccurred())

							ownerRef := metav1.OwnerReference{
								APIVersion:         "fissile.cloudfoundry.org/v1alpha1",
								Kind:               "ExtendedStatefulSet",
								Name:               ess.Name,
								UID:                ess.UID,
								Controller:         helper.Bool(false),
								BlockOwnerDeletion: helper.Bool(true),
							}

							for _, obj := range []apis.Object{configMap1, configMap2, secret1, secret2} {
								Expect(obj.GetOwnerReferences()).Should(ContainElement(ownerRef))
							}
						})

						By("Updates the config hash in the StatefulSet Annotations", func() {
							Expect(currentSHA1).ShouldNot(Equal(originalSHA1))
						})
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

		Context("when there are three versions", func() {
			var (
				currentSha string

				desiredExtendedStatefulSet *exss.ExtendedStatefulSet
				v1StatefulSet              *v1beta2.StatefulSet
				v2StatefulSet              *v1beta2.StatefulSet
				v3StatefulSet              *v1beta2.StatefulSet
				v1Pod                      *corev1.Pod
				v2Pod                      *corev1.Pod
				v3Pod                      *corev1.Pod
			)

			BeforeEach(func() {
				currentSha = "00654e00b9b5e4e4268e16392723c8b7fcfbab1b"
				desiredExtendedStatefulSet = &exss.ExtendedStatefulSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
						UID:       "foo-uid",
					},
					Spec: exss.ExtendedStatefulSetSpec{
						Template: v1beta2.StatefulSet{
							Spec: v1beta2.StatefulSetSpec{
								Replicas: helper.Int32(1),
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
				v2StatefulSet = &v1beta2.StatefulSet{
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
				v3StatefulSet = &v1beta2.StatefulSet{
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
				err := client.Update(context.Background(), v2Pod)
				Expect(err).ToNot(HaveOccurred())

				v3Pod.Status = corev1.PodStatus{
					Conditions: []corev1.PodCondition{
						{
							Type:   corev1.PodReady,
							Status: corev1.ConditionTrue,
						},
					},
				}
				err = client.Update(context.Background(), v3Pod)
				Expect(err).ToNot(HaveOccurred())

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta2.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
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
				err := client.Update(context.Background(), v2Pod)
				Expect(err).ToNot(HaveOccurred())

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5 * time.Second,
				}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta2.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).To(HaveOccurred())
				Expect(kerrors.IsNotFound(err)).To(BeTrue())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())

				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})

			It("does nothing and continues to reconcile when V1, V2 and V3 are all not running", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{
					Requeue:      true,
					RequeueAfter: 5 * time.Second,
				}))

				ess := &exss.ExtendedStatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, ess)
				Expect(err).ToNot(HaveOccurred())

				ss := &v1beta2.StatefulSet{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v1", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())
				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v2", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())
				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())

				err = client.Get(context.Background(), types.NamespacedName{Name: "foo-v3", Namespace: "default"}, ss)
				Expect(err).ToNot(HaveOccurred())
				Expect(metav1.IsControlledBy(ss, ess)).To(BeTrue())
			})
		})
	})
})
