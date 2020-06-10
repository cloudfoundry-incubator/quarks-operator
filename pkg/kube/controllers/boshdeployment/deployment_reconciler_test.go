package boshdeployment_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/boshdeployment"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	qsv1a1 "code.cloudfoundry.org/quarks-secret/pkg/kube/apis/quarkssecret/v1alpha1"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileBoshDeployment", func() {
	var (
		manager        *fakes.FakeManager
		reconciler     reconcile.Reconciler
		recorder       *record.FakeRecorder
		request        reconcile.Request
		ctx            context.Context
		withops        fakes.FakeWithOps
		jobFactory     fakes.FakeJobFactory
		kubeConverter  fakes.FakeVariablesConverter
		manifest       *bdm.Manifest
		log            *zap.SugaredLogger
		config         *cfcfg.Config
		client         *fakes.FakeClient
		instance       *bdv1.BOSHDeployment
		dmQJob         *qjv1a1.QuarksJob
		igQJob         *qjv1a1.QuarksJob
		deploymentName string
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		recorder = record.NewFakeRecorder(20)
		manager = &fakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)
		manager.GetEventRecorderForReturns(recorder)
		withops = fakes.FakeWithOps{}
		jobFactory = fakes.FakeJobFactory{}
		kubeConverter = fakes.FakeVariablesConverter{}
		kubeConverter.VariablesReturns([]qsv1a1.QuarksSecret{}, nil)

		deploymentName = "foo"

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: deploymentName, Namespace: "default"}}

		manifest = &bdm.Manifest{
			Releases: []*bdm.Release{
				{
					Name:    "bar",
					URL:     "docker.io/cfcontainerization",
					Version: "1.0",
					Stemcell: &bdm.ReleaseStemcell{
						OS:      "opensuse",
						Version: "42.3",
					},
				},
			},
			InstanceGroups: []*bdm.InstanceGroup{
				{
					Name: "fakepod",
					Jobs: []bdm.Job{
						{
							Name:    "foo",
							Release: "bar",
							Properties: bdm.JobProperties{
								Properties: map[string]interface{}{
									"password": "((foo_password))",
								},
								Quarks: bdm.Quarks{
									Ports: []bdm.Port{
										{
											Name:     "foo",
											Protocol: "TCP",
											Internal: 8080,
										},
									},
								},
							},
						},
					},
				},
			},
			Variables: []bdm.Variable{
				{
					Name: "foo_password",
					Type: "password",
				},
			},
		}
		dmQJob = &qjv1a1.QuarksJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("dm-%s", deploymentName),
				Namespace: "default",
			},
		}
		igQJob = &qjv1a1.QuarksJob{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("ig-%s", deploymentName),
				Namespace: "default",
			},
		}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		ctx = ctxlog.NewContextWithRecorder(ctx, "TestRecorder", recorder)

		instance = &bdv1.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: "default",
			},
			Spec: bdv1.BOSHDeploymentSpec{
				Manifest: bdv1.ResourceReference{
					Name: "dummy-manifest",
					Type: "configmap",
				},
				Ops: []bdv1.ResourceReference{
					{
						Name: "bar",
						Type: "configmap",
					},
					{
						Name: "baz",
						Type: "secret",
					},
				},
			},
		}

		client = &fakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *bdv1.BOSHDeployment:
				instance.DeepCopyInto(object)
			case *qjv1a1.QuarksJob:
				return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
			}

			return nil
		})
		client.StatusCalls(func() crc.StatusWriter { return &fakes.FakeStatusWriter{} })
		manager.GetClientReturns(client)

		jobFactory.VariableInterpolationJobReturns(dmQJob, nil)
		jobFactory.InstanceGroupManifestJobReturns(igQJob, nil)
	})

	JustBeforeEach(func() {
		withops.ManifestReturns(manifest, []string{}, nil)
		reconciler = cfd.NewDeploymentReconciler(
			ctx, config, manager,
			&withops, &jobFactory, &kubeConverter,
			controllerutil.SetControllerReference,
		)
	})

	Describe("Reconcile", func() {
		Context("when the manifest can not be resolved", func() {
			It("returns an empty result when the resource was not found", func() {
				client.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, "not found is requeued"))

				reconciler.Reconcile(request)
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(reconcile.Result{}).To(Equal(result))
			})

			It("handles an error when the request failed", func() {
				client.GetReturns(apierrors.NewBadRequest("bad request returns error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bad request returns error"))

				// check for events
				Expect(<-recorder.Events).To(ContainSubstring("GetBOSHDeploymentError"))
			})

			It("handles an error when resolving the BOSHDeployment", func() {
				withops.ManifestReturns(nil, []string{}, fmt.Errorf("resolver error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("resolver error"))

				// check for events
				Expect(<-recorder.Events).To(ContainSubstring("WithOpsManifestError"))
			})
		})

		Context("when the manifest can be resolved", func() {
			It("handles an error when resolving manifest", func() {
				manifest = &bdm.Manifest{}
				withops.ManifestReturns(manifest, []string{}, errors.New("fake-error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error resolving the manifest 'default/foo': fake-error"))
			})

			It("handles an error when setting the owner reference on the object", func() {
				reconciler = cfd.NewDeploymentReconciler(ctx, config, manager, &withops, &jobFactory, &kubeConverter,
					func(owner, object metav1.Object, scheme *runtime.Scheme) error {
						return fmt.Errorf("some error")
					},
				)

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to set ownerReference for Secret 'default/with-ops': some error"))
			})

			It("handles an error when creating manifest secret with ops", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object)
					case *qjv1a1.QuarksJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					case *corev1.Secret:
						if nn.Name == "with-ops" {
							return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
						}
					}

					return nil
				})
				client.UpdateCalls(func(context context.Context, object runtime.Object, _ ...crc.UpdateOption) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						object.DeepCopyInto(instance)
					}
					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
					switch object.(type) {
					case *corev1.Secret:
						return errors.New("fake-error")
					}
					return nil
				})

				By("From created state to ops applied state")
				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create with-ops manifest secret for BOSHDeployment 'default/foo': failed to apply Secret 'default/with-ops': fake-error"))
			})

			It("handles an error when building desired manifest qJob", func() {
				jobFactory.VariableInterpolationJobReturns(dmQJob, errors.New("fake-error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to build the desired manifest qJob"))
			})

			It("handles an error generating the new variable secrets", func() {
				kubeConverter.VariablesReturns(nil, errors.New("fake-error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to generate quarks secrets from manifest"))
			})

			It("handles an error when creating the new quarks secrets", func() {
				kubeConverter.VariablesReturns([]qsv1a1.QuarksSecret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "fake-variable",
						},
					},
				}, nil)
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object)
					case *qsv1a1.QuarksSecret:
						return apierrors.NewNotFound(schema.GroupResource{}, "fake-variable")
					}
					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
					switch object.(type) {
					case *qsv1a1.QuarksSecret:
						return errors.New("fake-error")
					}
					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create quarks secrets for BOSH manifest 'default/foo'"))
			})

			It("handles an error when building desired manifest qJob", func() {
				jobFactory.VariableInterpolationJobReturns(dmQJob, errors.New("fake-error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to build the desired manifest qJob"))
			})

			It("handles an error when creating desired manifest qJob", func() {
				client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
					switch object := object.(type) {
					case *qjv1a1.QuarksJob:
						qJob := object
						if strings.HasPrefix(qJob.Name, "dm-") {
							return errors.New("fake-error")
						}
					}
					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create desired manifest qJob for BOSHDeployment 'default/foo': creating or updating QuarksJob 'default/dm-foo': fake-error"))
			})

			It("handles an error when building instance group manifest qJob", func() {
				jobFactory.InstanceGroupManifestJobReturns(dmQJob, errors.New("fake-error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to build instance group manifest qJob"))
			})

			It("handles an error when creating instance group manifest qJob", func() {
				client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
					switch object := object.(type) {
					case *qjv1a1.QuarksJob:
						qJob := object
						if strings.HasPrefix(qJob.Name, "ig-") {
							return errors.New("fake-error")
						}
					}
					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create instance group manifest qJob for BOSHDeployment 'default/foo': creating or updating QuarksJob 'default/ig-foo': fake-error"))
			})

			Context("when the manifest contains variables", func() {
				BeforeEach(func() {
					kubeConverter.VariablesReturns([]qsv1a1.QuarksSecret{
						{ObjectMeta: metav1.ObjectMeta{Name: "fake-variable", Namespace: "default"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "other-variable", Namespace: "default"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "last-variable", Namespace: "default"}},
					}, nil)
					client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
						switch object := object.(type) {
						case *bdv1.BOSHDeployment:
							instance.DeepCopyInto(object)
						case *qjv1a1.QuarksJob:
							return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
						case *qsv1a1.QuarksSecret:
							return apierrors.NewNotFound(schema.GroupResource{}, "")
						}
						return nil
					})
				})

				It("creates the variable secrets", func() {
					result, err := reconciler.Reconcile(request)
					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
					Expect(client.CreateCallCount()).To(Equal(5))
				})
			})

			Context("when the manifest contains explicit links to native k8s resources", func() {
				var bazSecret *corev1.Secret

				BeforeEach(func() {
					bazSecret = &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "baz-sec",
							Namespace: "default",
							Annotations: map[string]string{
								bdv1.LabelDeploymentName:       deploymentName,
								bdv1.AnnotationLinkProvidesKey: `{"name":"baz"}`,
							},
						},
						Data: map[string][]byte{},
					}

					manifest = &bdm.Manifest{
						Releases: []*bdm.Release{
							{
								Name:    "bar",
								URL:     "docker.io/cfcontainerization",
								Version: "1.0",
								Stemcell: &bdm.ReleaseStemcell{
									OS:      "opensuse",
									Version: "42.3",
								},
							},
						},
						InstanceGroups: []*bdm.InstanceGroup{
							{
								Name: "fakepod",
								Jobs: []bdm.Job{
									{
										Name:    "foo",
										Release: "bar",
										Properties: bdm.JobProperties{
											Properties: map[string]interface{}{
												"password": "((foo_password))",
											},
											Quarks: bdm.Quarks{
												Ports: []bdm.Port{
													{
														Name:     "foo",
														Protocol: "TCP",
														Internal: 8080,
													},
												},
											},
										},
										Consumes: map[string]interface{}{
											"baz": map[string]interface{}{
												"from": "baz",
											},
										},
									},
								},
							},
						},
						Variables: []bdm.Variable{
							{
								Name: "foo_password",
								Type: "password",
							},
						},
					}

					client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
						switch object := object.(type) {
						case *corev1.SecretList:
							secretList := corev1.SecretList{
								Items: []corev1.Secret{*bazSecret},
							}
							secretList.DeepCopyInto(object)
						}

						return nil
					})
				})

				It("passes link secrets to QJobs", func() {
					_, err := reconciler.Reconcile(request)
					Expect(err).ToNot(HaveOccurred())
					_, _, _, linksSecrets, _ := jobFactory.InstanceGroupManifestJobArgsForCall(0)
					Expect(linksSecrets).To(Equal(converter.LinkInfos{
						{
							SecretName:   "baz-sec",
							ProviderName: "baz",
						},
					}))
					_, _, _, linksSecrets, _ = jobFactory.InstanceGroupManifestJobArgsForCall(0)
					Expect(linksSecrets).To(Equal(converter.LinkInfos{
						{
							SecretName:   "baz-sec",
							ProviderName: "baz",
						},
					}))
				})

				It("handles an error when listing secrets", func() {
					client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
						switch object.(type) {
						case *corev1.SecretList:
							return errors.New("fake-error")
						}

						return nil
					})

					_, err := reconciler.Reconcile(request)
					Expect(err.Error()).To(ContainSubstring("listing secrets for link in deployment"))
				})

				It("handles an error on missing providers when the secret doesn't have the annotation", func() {
					bazSecret.Annotations = nil
					_, err := reconciler.Reconcile(request)
					Expect(err.Error()).To(ContainSubstring("missing link secrets for providers"))
				})

				It("handles an error on duplicated secrets of provider when duplicated secrets match the annotation", func() {
					client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
						switch object := object.(type) {
						case *corev1.SecretList:
							secretList := corev1.SecretList{
								Items: []corev1.Secret{*bazSecret, *bazSecret},
							}
							secretList.DeepCopyInto(object)
						}

						return nil
					})

					_, err := reconciler.Reconcile(request)
					Expect(err.Error()).To(ContainSubstring("duplicated secrets of provider"))
				})
			})
		})
	})
})
