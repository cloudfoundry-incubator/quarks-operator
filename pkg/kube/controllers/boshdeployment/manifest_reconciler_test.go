package boshdeployment_test

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

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	store "code.cloudfoundry.org/cf-operator/pkg/kube/util/store/manifest"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("ReconcileManifest", func() {
	var (
		desiredManifestSecretName string
		desiredManifestSecret     *corev1.Secret
		instance                  *bdv1.BOSHDeployment
		dataGatheringJob          *ejv1.ExtendedJob

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

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		instance = &bdv1.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: bdv1.BOSHDeploymentSpec{},
		}

		_, desiredManifestSecretName = bdm.CalculateEJobOutputSecretPrefixAndName(bdm.DeploymentSecretTypeManifestAndVars, instance.GetName(), bdm.VarInterpolationContainerName)
		desiredManifestSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      desiredManifestSecretName + "-v1",
				Namespace: "default",
				UID:       "",
				Labels: map[string]string{
					bdv1.LabelDeploymentName: "fake-manifest",
					store.LabelKind:          "manifest",
					store.LabelVersion:       "1",
				},
			},
		}

		eJobName := fmt.Sprintf("data-gathering-%s", instance.Name)
		dataGatheringJob = &ejv1.ExtendedJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      eJobName,
				Namespace: "default",
				UID:       "",
			},
			Spec: ejv1.ExtendedJobSpec{
				Output: &ejv1.Output{
					NamePrefix: "fake-manifest.ig-resolved",
					SecretLabels: map[string]string{
						bdv1.LabelDeploymentName: "fake-manifest",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Name: eJobName,
					},
					Spec: corev1.PodSpec{
						// Volumes for secrets
						Volumes: []corev1.Volume{
							{
								Name: "fake-manifest-volume",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: desiredManifestSecretName,
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
		reconciler = cfd.NewManifestReconciler(ctx, config, manager)
	})

	Describe("Reconcile", func() {
		Context("when the desired manifest is created", func() {
			var (
				client *cfakes.FakeClient
			)
			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				manager.GetClientReturns(client)
			})

			It("updates volumes for desired manifest secret", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dataGatheringJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						desiredManifestSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				client.UpdateCalls(func(_ context.Context, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						extendedJob := object.(*ejv1.ExtendedJob)
						Expect(extendedJob.Spec.Template.Spec.Volumes).To(ContainElement(corev1.Volume{
							Name: "fake-manifest-volume",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: desiredManifestSecretName + "-v1",
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

			It("returns an error if a bad request error occurs when fetching the BOSHDeployment instance", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						return apierrors.NewBadRequest("fake-error")
					}
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not get BOSHDeployment"))
				Expect(result).To(Equal(reconcile.Result{}))

			})

			It("Reconciles if the data gathering job can not be found", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("data gathering job '%s' not found", dataGatheringJob.Name)))
				Expect(result).To(Equal(reconcile.Result{
					Requeue: true,
				}))

			})

			It("returns an error if a bad request error occurs when fetching the data gathering job", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						return apierrors.NewBadRequest("fake-error")
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("could not get data gathering job '%s'", dataGatheringJob.Name)))
				Expect(result).To(Equal(reconcile.Result{}))

			})

			It("returns an error if a bad request error occurs when fetching the manifest secret job", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dataGatheringJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						return apierrors.NewBadRequest("fake-error")
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
						return nil
					}

					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})

				result, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("could not to get manifest secret for '%s'", desiredManifestSecretName)))
				Expect(result).To(Equal(reconcile.Result{}))

			})

			It("returns an error if a bad request error occurs when updating data gathering job", func() {
				client.GetCalls(func(_ context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						dataGatheringJob.DeepCopyInto(object.(*ejv1.ExtendedJob))
						return nil
					case *corev1.Secret:
						desiredManifestSecret.DeepCopyInto(object.(*corev1.Secret))
						return nil
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
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
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("could not update ExtendedJob '%s/%s'", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName())))
				Expect(result).To(Equal(reconcile.Result{}))

			})
		})
	})
})
