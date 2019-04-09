package extendedsecret_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	. "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("ReconcileVersionedSecret", func() {
	var (
		dependantSecretName string
		dependantJob        *ejv1.ExtendedJob
		versionedSecret     *corev1.Secret

		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request

		log    *zap.SugaredLogger
		config *cfcfg.Config
		ctx    context.Context
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: dependantSecretName + "-v1", Namespace: "default"}}

		dependantJobName := "fake-job"
		dependantSecretName = "foo"

		versionedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dependantSecretName + "-v1",
				Namespace: "default",
				UID:       "",
				Labels: map[string]string{
					bdv1.LabelDeploymentName:      "fake-deployment",
					ejv1.LabelDependantJobName:    dependantJobName,
					ejv1.LabelDependantSecretName: dependantSecretName,
					LabelSecretKind:               "versionedSecret",
					LabelVersion:                  "1",
				},
			},
		}

		dependantJob = &ejv1.ExtendedJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      dependantJobName,
				Namespace: "default",
				UID:       "",
			},
			Spec: ejv1.ExtendedJobSpec{
				Output: &ejv1.Output{
					NamePrefix: "fake-output",
					SecretLabels: map[string]string{
						bdv1.LabelDeploymentName: "fake-deployment",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: dependantJobName,
					},
					Spec: corev1.PodSpec{
						// Volumes for secrets
						Volumes: []corev1.Volume{
							{
								Name: "fake-secret-volume",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: dependantSecretName,
									},
								},
							},
						},
					},
				},
			},
		}

		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewManagerContext(log)
	})

	JustBeforeEach(func() {
		reconciler = NewVersionedSecretReconciler(ctx, config, manager)
	})

	Describe("Reconcile", func() {
		Context("when the versioned secret is created", func() {
			var (
				client *cfakes.FakeClient
			)
			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				manager.GetClientReturns(client)
			})

			It("updates volumes for versioned secret", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dependantJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				client.UpdateCalls(func(_ context.Context, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						extendedJob := object.(*ejv1.ExtendedJob)
						Expect(extendedJob.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
							Name: "fake-secret-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: dependantSecretName + "-v1",
								},
							},
						}))
						return nil
					}

					return nil
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("skips reconcile if the instance can not be found", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

			})

			It("reconciles if the versioned secret does not have labels", func() {
				versionedSecret.Labels = nil

				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("secret '%s' does not have labels", versionedSecret.GetName())))
				Expect(result).To(Equal(reconcile.Result{
					Requeue: true,
				}))
			})

			It("reconciles if the versioned secret does not have required labels", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				delete(versionedSecret.Labels, ejv1.LabelDependantSecretName)

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("label '%s' not found in versioned secret '%s'", ejv1.LabelDependantSecretName, versionedSecret.GetName())))
				Expect(result).To(Equal(reconcile.Result{
					Requeue: true,
				}))

				delete(versionedSecret.Labels, ejv1.LabelDependantJobName)

				result, err = reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("label '%s' not found in versioned secret '%s'", ejv1.LabelDependantJobName, versionedSecret.GetName())))
				Expect(result).To(Equal(reconcile.Result{
					Requeue: true,
				}))
			})

			It("reconciles if the dependant job can not be found", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					case *ejv1.ExtendedJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("dependant job '%s' not found", dependantJob.Name)))
				Expect(result).To(Equal(reconcile.Result{
					Requeue: true,
				}))
			})

			It("reconciles if a bad request error occurs when fetching the dependant job", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					case *ejv1.ExtendedJob:
						return apierrors.NewBadRequest("fake-error")
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("could not get dependant job '%s'", dependantJob.Name)))
				Expect(result).To(Equal(reconcile.Result{
					Requeue: true,
				}))
			})

			It("returns an error if the versioned secret does not have version label", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dependantJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				delete(versionedSecret.Labels, LabelVersion)

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("label '%s' found in versioned secret '%s'", ejv1.LabelDependantSecretName, versionedSecret.GetName())))
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("returns an error if convert version string error occurs", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dependantJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				versionStr := "fake"
				versionedSecret.Labels[LabelVersion] = versionStr

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("could not convert version '%s' to int", versionStr)))
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("returns an error if a bad request error occurs when fetching the versioned secret", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dependantJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						return apierrors.NewBadRequest("fake-error")
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("could not get secret '%s'", request.NamespacedName)))
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("returns an error if a bad request error occurs when updating dependant job", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dependantJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						versionedSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				client.UpdateCalls(func(_ context.Context, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						return apierrors.NewBadRequest("fake-error")
					}

					return nil
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("could not update ExtendedJob '%s/%s'", dependantJob.GetNamespace(), dependantJob.GetName())))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
