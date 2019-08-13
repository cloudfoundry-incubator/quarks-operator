package mutate_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"
)

var _ = Describe("Mutate", func() {
	var (
		ctx    context.Context
		client *cfakes.FakeClient
	)

	BeforeEach(func() {
		client = &cfakes.FakeClient{}
	})

	Describe("BoshDeploymentMutateFn", func() {
		var (
			boshDeployment *bdv1.BOSHDeployment
		)

		BeforeEach(func() {
			boshDeployment = &bdv1.BOSHDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: bdv1.BOSHDeploymentSpec{
					Manifest: bdv1.ResourceReference{
						Name: "dummy-manifest",
						Type: bdv1.ConfigMapReference,
					},
				},
			}
		})

		Context("when the boshDeployment is not found", func() {
			It("creates the BoshDeployment", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, boshDeployment, mutate.BoshDeploymentMutateFn(boshDeployment))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the boshDeployment is found", func() {
			It("updates the BoshDeployment when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						existing := &bdv1.BOSHDeployment{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: bdv1.BOSHDeploymentSpec{
								Manifest: bdv1.ResourceReference{
									Name: "initial-manifest",
									Type: bdv1.ConfigMapReference,
								},
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, boshDeployment, mutate.BoshDeploymentMutateFn(boshDeployment))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update the BoshDeployment when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						boshDeployment.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, boshDeployment, mutate.BoshDeploymentMutateFn(boshDeployment))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})

	Describe("EStsMutateFn", func() {
		var (
			eSts *essv1.ExtendedStatefulSet
		)

		BeforeEach(func() {
			eSts = &essv1.ExtendedStatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: essv1.ExtendedStatefulSetSpec{
					Template: appsv1.StatefulSet{
						Spec: appsv1.StatefulSetSpec{
							Replicas: util.Int32(1),
						},
					},
				},
			}
		})

		Context("when the extendedStatefulSet is not found", func() {
			It("creates the extendedStatefulSet", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSts, mutate.EStsMutateFn(eSts))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the extendedStatefulSet is found", func() {
			It("updates the extendedStatefulSet when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *essv1.ExtendedStatefulSet:
						existing := &essv1.ExtendedStatefulSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: essv1.ExtendedStatefulSetSpec{
								Template: appsv1.StatefulSet{
									Spec: appsv1.StatefulSetSpec{
										Replicas: util.Int32(2),
									},
								},
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSts, mutate.EStsMutateFn(eSts))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update the extendedStatefulSet when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *essv1.ExtendedStatefulSet:
						object = eSts.DeepCopy()

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSts, mutate.EStsMutateFn(eSts))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})

	Describe("EJobMutateFn", func() {
		var (
			eJob *ejv1.ExtendedJob
		)

		BeforeEach(func() {
			eJob = &ejv1.ExtendedJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: ejv1.ExtendedJobSpec{
					Trigger: ejv1.Trigger{
						Strategy: ejv1.TriggerOnce,
					},
					UpdateOnConfigChange: true,
				},
			}
		})

		Context("when the extendedJob is not found", func() {
			It("creates the extendedJob", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, eJob, mutate.EJobMutateFn(eJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the extendedJob is found", func() {
			It("updates the extendedJob when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *ejv1.ExtendedJob:
						existing := &ejv1.ExtendedJob{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: ejv1.ExtendedJobSpec{},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eJob, mutate.EJobMutateFn(eJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update the extendedJob when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						object = eJob.DeepCopy()

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eJob, mutate.EJobMutateFn(eJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})

			It("does not update trigger strategy", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *ejv1.ExtendedJob:
						existing := &ejv1.ExtendedJob{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: ejv1.ExtendedJobSpec{
								Trigger: ejv1.Trigger{
									Strategy: ejv1.TriggerNow,
								},
								UpdateOnConfigChange: true,
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eJob, mutate.EJobMutateFn(eJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})

	Describe("ESecMutateFn", func() {
		var (
			eSec *esv1.ExtendedSecret
		)

		BeforeEach(func() {
			eSec = &esv1.ExtendedSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: esv1.ExtendedSecretSpec{
					Type:       esv1.Password,
					SecretName: "dummy-secret",
				},
			}
		})

		Context("when the extendedSecret is not found", func() {
			It("creates the extendedSecret", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSec, mutate.ESecMutateFn(eSec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the extendedSecret is found", func() {
			It("updates the extendedSecret when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *esv1.ExtendedSecret:
						existing := &esv1.ExtendedSecret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: esv1.ExtendedSecretSpec{
								Type:       esv1.Password,
								SecretName: "initial-secret",
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSec, mutate.ESecMutateFn(eSec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update the extendedSecret when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *esv1.ExtendedSecret:
						object = eSec.DeepCopy()

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSec, mutate.ESecMutateFn(eSec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})

	Describe("SecretMutateFn", func() {
		var (
			sec *corev1.Secret
		)

		BeforeEach(func() {
			sec = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				StringData: map[string]string{
					"dummy": "foo-value",
				},
			}
		})

		Context("when the secret is not found", func() {
			It("creates the secret", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, sec, mutate.SecretMutateFn(sec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the secret is found", func() {
			It("updates the secret when secret data is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						existing := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"dummy": []byte("initial-value"),
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, sec, mutate.SecretMutateFn(sec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update the secret when secret data is not changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						existing := &corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Data: map[string][]byte{
								"dummy": []byte("foo-value"),
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, sec, mutate.SecretMutateFn(sec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})

	Describe("ServiceMutateFn", func() {
		var (
			svc *corev1.Service
		)

		BeforeEach(func() {
			svc = &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:     "exposed-port",
							Protocol: corev1.ProtocolTCP,
							Port:     8080,
						},
					},
					Selector: map[string]string{
						"foo": "bar",
					},
				},
			}
		})

		Context("when the service is not found", func() {
			It("creates the service", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, svc, mutate.ServiceMutateFn(svc))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the service is found", func() {
			It("updates the service when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Service:
						existing := &corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: corev1.ServiceSpec{
								Ports: []corev1.ServicePort{
									{
										Name:     "initial-exposed-port",
										Protocol: corev1.ProtocolTCP,
										Port:     8080,
									},
								},
								Selector: map[string]string{
									"foo": "bar",
								},
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, svc, mutate.ServiceMutateFn(svc))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update cluster IP", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Service:
						existing := &corev1.Service{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: corev1.ServiceSpec{
								ClusterIP: "10.10.10.10",
								Ports: []corev1.ServicePort{
									{
										Name:     "exposed-port",
										Protocol: corev1.ProtocolTCP,
										Port:     8080,
									},
								},
								Selector: map[string]string{
									"foo": "bar",
								},
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, svc, mutate.ServiceMutateFn(svc))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})
})
