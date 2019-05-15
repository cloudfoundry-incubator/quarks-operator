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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/fakes"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	ctxlog "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("ReconcileBoshDeployment", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		recorder   *record.FakeRecorder
		request    reconcile.Request
		ctx        context.Context
		resolver   fakes.FakeResolver
		manifest   *bdm.Manifest
		log        *zap.SugaredLogger
		config     *cfcfg.Config
		client     *cfakes.FakeClient
		instance   *bdv1.BOSHDeployment
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		recorder = record.NewFakeRecorder(20)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)
		manager.GetRecorderReturns(recorder)
		resolver = fakes.FakeResolver{}

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		manifest = &bdm.Manifest{
			Name: "fake-manifest",
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
								BOSHContainerization: bdm.BOSHContainerization{
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
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		ctx = ctxlog.NewContextWithRecorder(ctx, "TestRecorder", recorder)

		instance = &bdv1.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: bdv1.BOSHDeploymentSpec{
				Manifest: bdv1.Manifest{
					Ref:  "dummy-manifest",
					Type: "configmap",
				},
				Ops: []bdv1.Ops{
					{
						Ref:  "bar",
						Type: "configmap",
					},
					{
						Ref:  "baz",
						Type: "secret",
					},
				},
			},
		}

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object.(type) {
			case *bdv1.BOSHDeployment:
				instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
			}

			return nil
		})

		manager.GetClientReturns(client)
	})

	JustBeforeEach(func() {
		resolver.ResolveManifestReturns(manifest, nil)
		reconciler = cfd.NewDeploymentReconciler(ctx, config, manager, &resolver,
			controllerutil.SetControllerReference,
			versionedsecretstore.NewVersionedSecretStore(manager.GetClient()),
			bdm.NewKubeConverter(config.Namespace),
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
				resolver.ResolveManifestReturns(nil, fmt.Errorf("resolver error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("resolver error"))

				// check for events
				Expect(<-recorder.Events).To(ContainSubstring("ResolveManifestError"))
			})

			It("handles an error when missing instance groups", func() {
				resolver.ResolveManifestReturns(&bdm.Manifest{
					InstanceGroups: []*bdm.InstanceGroup{},
				}, nil)

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no instance groups defined in manifest"))

				// check for events
				Expect(<-recorder.Events).To(ContainSubstring("MissingInstanceError"))
			})
		})

		Context("when the manifest can be resolved", func() {
			It("handles an error when resolving manifest", func() {
				manifest = &bdm.Manifest{}
				resolver.ResolveManifestReturns(manifest, errors.New("fake-error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not resolve manifest"))
			})

			It("handles an error when setting the owner reference on the object", func() {
				reconciler = cfd.NewDeploymentReconciler(ctx, config, manager, &resolver,
					func(owner, object metav1.Object, scheme *runtime.Scheme) error {
						return fmt.Errorf("failed to set reference")
					},
					versionedsecretstore.NewVersionedSecretStore(manager.GetClient()),
					bdm.NewKubeConverter(config.Namespace),
				)

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not set specs ownerReference"))
			})

			It("handles an error when setting finalizer for BOSHDeployment", func() {
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						return apierrors.NewNotFound(schema.GroupResource{}, "not found is requeued")
					}
					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not set instance's finalizer"))
			})

			It("handles an error when converting bosh manifest to kube objects", func() {
				resolver.ResolveManifestReturns(&bdm.Manifest{
					InstanceGroups: []*bdm.InstanceGroup{
						{
							Name: "empty-instance",
						},
					},
				}, nil)

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error converting bosh manifest"))
			})

			It("handles an errors when getting instance state", func() {
				getInstanceCallCount := 0
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
						getInstanceCallCount++
						// We have 3 times calls to reach into r.updateInstanceState
						if getInstanceCallCount > 2 {
							return errors.New("fake-error")
						}
					}

					return nil
				})
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
					}
					return nil
				})

				By("From created state to ops applied state")
				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not get BOSHDeployment instance"))
			})

			It("handles an errors when updating instance state", func() {
				updateInstanceCallCount := 0
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
						updateInstanceCallCount++
						// We have 2 times calls to reach into r.updateInstanceState
						if updateInstanceCallCount > 1 {
							return errors.New("fake-error")
						}
					}
					return nil
				})

				By("From created state to ops applied state")
				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("could not update BOSHDeployment instance"))
			})

			It("handles an errors when creating manifest secret with ops ", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
					case *ejv1.ExtendedJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					case *corev1.Secret:
						if nn.Name == "foo.with-ops" {
							return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
						}
					}

					return nil
				})
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
					}
					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *corev1.Secret:
						return errors.New("fake-error")
					}
					return nil
				})

				By("From created state to ops applied state")
				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create manifest with ops"))
			})

			It("handles an errors when creating variable interpolation eJob", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
					case *ejv1.ExtendedJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
					}
					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						eJob := object.(*ejv1.ExtendedJob)
						if strings.HasPrefix(eJob.Name, "var-interpolation") {
							return errors.New("fake-error")
						}
					}
					return nil
				})

				By("From created state to ops applied state")
				result, err := reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				instance := &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, instance)
				Expect(err).ToNot(HaveOccurred())
				Expect(instance.Status.State).To(Equal(cfd.OpsAppliedState))

				// Simulate generated variable reconciliation
				instance.Status.State = cfd.VariableGeneratedState
				err = client.Update(context.Background(), instance)
				Expect(err).ToNot(HaveOccurred())

				By("From ops applied state to ops variable generated state")
				result, err = reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create variable interpolation eJob"))
			})

			It("handles an errors when creating data gathering eJob", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
					case *ejv1.ExtendedJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
					}
					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *ejv1.ExtendedJob:
						eJob := object.(*ejv1.ExtendedJob)
						if strings.HasPrefix(eJob.Name, "data-gathering") {
							return errors.New("fake-error")
						}
					}
					return nil
				})

				By("From created state to ops applied state")
				result, err := reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				instance := &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, instance)
				Expect(err).ToNot(HaveOccurred())
				Expect(instance.Status.State).To(Equal(cfd.OpsAppliedState))

				// Simulate generated variable reconciliation
				instance.Status.State = cfd.VariableGeneratedState
				err = client.Update(context.Background(), instance)
				Expect(err).ToNot(HaveOccurred())

				By("From ops applied state to variable interpolated state")
				result, err = reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				instance = &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, instance)
				Expect(err).ToNot(HaveOccurred())
				Expect(instance.Status.State).To(Equal(cfd.VariableInterpolatedState))

				By("From variable generated state to data gathered state")
				result, err = reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create data gathering eJob"))
			})

			It("goes from created state to data gathered state successfully", func() {
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
					}
					return nil
				})

				By("From created state to ops applied state")
				result, err := reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				instance := &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, instance)
				Expect(err).ToNot(HaveOccurred())
				Expect(instance.Status.State).To(Equal(cfd.OpsAppliedState))

				// Simulate generated variable reconciliation
				instance.Status.State = cfd.VariableGeneratedState
				err = client.Update(context.Background(), instance)
				Expect(err).ToNot(HaveOccurred())

				By("From variable generated state to variable interpolated state")
				result, err = reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				instance = &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, instance)
				Expect(err).ToNot(HaveOccurred())
				Expect(instance.Status.State).To(Equal(cfd.VariableInterpolatedState))

				By("From variable interpolated state to data gathered state")
				result, err = reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				instance = &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, instance)
				Expect(err).ToNot(HaveOccurred())
				Expect(instance.Status.State).To(Equal(cfd.DataGatheredState))
			})
		})
	})
})
