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
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/mutate"
	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
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
			eSts *qstsv1a1.QuarksStatefulSet
		)

		BeforeEach(func() {
			eSts = &qstsv1a1.QuarksStatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: qstsv1a1.QuarksStatefulSetSpec{
					Template: appsv1.StatefulSet{
						Spec: appsv1.StatefulSetSpec{
							Replicas: pointers.Int32(1),
						},
					},
				},
			}
		})

		Context("when the quarksStatefulSet is not found", func() {
			It("creates the quarksStatefulSet", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSts, mutate.EStsMutateFn(eSts))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the quarksStatefulSet is found", func() {
			It("updates the quarksStatefulSet when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *qstsv1a1.QuarksStatefulSet:
						existing := &qstsv1a1.QuarksStatefulSet{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: qstsv1a1.QuarksStatefulSetSpec{
								Template: appsv1.StatefulSet{
									Spec: appsv1.StatefulSetSpec{
										Replicas: pointers.Int32(2),
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

			It("does not update the quarksStatefulSet when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *qstsv1a1.QuarksStatefulSet:
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
			eSec *qsv1a1.QuarksSecret
		)

		BeforeEach(func() {
			eSec = &qsv1a1.QuarksSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: qsv1a1.QuarksSecretSpec{
					Type:       qsv1a1.Password,
					SecretName: "dummy-secret",
				},
			}
		})

		Context("when the quarksSecret is not found", func() {
			It("creates the quarksSecret", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSec, mutate.ESecMutateFn(eSec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the quarksSecret is found", func() {
			It("updates the quarksSecret when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *qsv1a1.QuarksSecret:
						existing := &qsv1a1.QuarksSecret{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: qsv1a1.QuarksSecretSpec{
								Type:       qsv1a1.Password,
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

			It("does not update the quarksSecret when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *qsv1a1.QuarksSecret:
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
