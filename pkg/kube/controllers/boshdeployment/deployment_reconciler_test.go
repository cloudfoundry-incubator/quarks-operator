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
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
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
			switch object := object.(type) {
			case *bdv1.BOSHDeployment:
				instance.DeepCopyInto(object)
			}

			return nil
		})

		manager.GetClientReturns(client)
	})

	JustBeforeEach(func() {
		resolver.WithOpsManifestReturns(manifest, nil)
		reconciler = cfd.NewDeploymentReconciler(ctx, config, manager,
			&resolver, controllerutil.SetControllerReference,
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
				resolver.WithOpsManifestReturns(nil, fmt.Errorf("resolver error"))

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
				resolver.WithOpsManifestReturns(manifest, errors.New("fake-error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error resolving the manifest foo: fake-error"))
			})

			It("handles an error when setting the owner reference on the object", func() {
				reconciler = cfd.NewDeploymentReconciler(ctx, config, manager, &resolver,
					func(owner, object metav1.Object, scheme *runtime.Scheme) error {
						return fmt.Errorf("some error")
					},
				)

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to set ownerReference for Secret 'foo.with-ops': some error"))
			})

			It("handles an errors when creating manifest secret with ops ", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object)
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
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						object.DeepCopyInto(instance)
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
				Expect(err.Error()).To(ContainSubstring("failed to create with-ops manifest secret for BOSHDeployment 'default/foo': failed to apply Secret 'foo.with-ops': fake-error"))
			})

			It("handles an errors when creating variable interpolation eJob", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object)
					case *ejv1.ExtendedJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						object.DeepCopyInto(instance)
					}
					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object := object.(type) {
					case *ejv1.ExtendedJob:
						eJob := object
						if strings.HasPrefix(eJob.Name, "dm-") {
							return errors.New("fake-error")
						}
					}
					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create variable interpolation ExtendedJob for BOSHDeployment 'default/foo': fake-error"))
			})

			It("handles an errors when creating data gathering eJob", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object)
					case *ejv1.ExtendedJob:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object := object.(type) {
					case *bdv1.BOSHDeployment:
						object.DeepCopyInto(instance)
					}
					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object := object.(type) {
					case *ejv1.ExtendedJob:
						eJob := object
						if strings.HasPrefix(eJob.Name, "dg-") {
							return errors.New("fake-error")
						}
					}
					return nil
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to create data gathering ExtendedJob for BOSHDeployment 'default/foo': fake-error"))
			})
		})
	})
})
