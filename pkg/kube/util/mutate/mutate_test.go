package mutate_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	qsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	qstsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	cfakes "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/mutate"
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

	Describe("QuarksStatefulSetMutateFn", func() {
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

				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSts, mutate.QuarksStatefulSetMutateFn(eSts))
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
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSts, mutate.QuarksStatefulSetMutateFn(eSts))
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
				ops, err := controllerutil.CreateOrUpdate(ctx, client, eSts, mutate.QuarksStatefulSetMutateFn(eSts))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})

	Describe("QuarksJobMutateFn", func() {
		var (
			qJob *qjv1a1.QuarksJob
		)

		BeforeEach(func() {
			qJob = &qjv1a1.QuarksJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
				Spec: qjv1a1.QuarksJobSpec{
					Trigger: qjv1a1.Trigger{
						Strategy: qjv1a1.TriggerOnce,
					},
					UpdateOnConfigChange: true,
				},
			}
		})

		Context("when the quarksJob is not found", func() {
			It("creates the quarksJob", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				ops, err := controllerutil.CreateOrUpdate(ctx, client, qJob, mutate.QuarksJobMutateFn(qJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultCreated))
			})
		})

		Context("when the quarksJob is found", func() {
			It("updates the quarksJob when spec is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *qjv1a1.QuarksJob:
						existing := &qjv1a1.QuarksJob{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: qjv1a1.QuarksJobSpec{},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, qJob, mutate.QuarksJobMutateFn(qJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update the quarksJob when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *qjv1a1.QuarksJob:
						object = qJob.DeepCopy()

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, qJob, mutate.QuarksJobMutateFn(qJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})

			It("does not update trigger strategy", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *qjv1a1.QuarksJob:
						existing := &qjv1a1.QuarksJob{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "foo",
								Namespace: "default",
							},
							Spec: qjv1a1.QuarksJobSpec{
								Trigger: qjv1a1.Trigger{
									Strategy: qjv1a1.TriggerNow,
								},
								UpdateOnConfigChange: true,
							},
						}
						existing.DeepCopyInto(object)

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, qJob, mutate.QuarksJobMutateFn(qJob))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultNone))
			})
		})
	})

	Describe("QuarksSecretMutateFn", func() {
		var (
			qSec *qsv1a1.QuarksSecret
		)

		BeforeEach(func() {
			qSec = &qsv1a1.QuarksSecret{
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

				ops, err := controllerutil.CreateOrUpdate(ctx, client, qSec, mutate.QuarksSecretMutateFn(qSec))
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
				ops, err := controllerutil.CreateOrUpdate(ctx, client, qSec, mutate.QuarksSecretMutateFn(qSec))
				Expect(err).ToNot(HaveOccurred())
				Expect(ops).To(Equal(controllerutil.OperationResultUpdated))
			})

			It("does not update the quarksSecret when nothing is changed", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *qsv1a1.QuarksSecret:
						object = qSec.DeepCopy()

						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
				ops, err := controllerutil.CreateOrUpdate(ctx, client, qSec, mutate.QuarksSecretMutateFn(qSec))
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
