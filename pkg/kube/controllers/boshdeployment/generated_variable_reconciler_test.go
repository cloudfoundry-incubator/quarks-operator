package boshdeployment_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("ReconcileGeneratedVariable", func() {
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
			Status: bdv1.BOSHDeploymentStatus{
				State: cfd.OpsAppliedState,
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
		reconciler = cfd.NewGeneratedVariableReconciler(ctx, config, manager, &resolver, controllerutil.SetControllerReference)
	})

	Describe("Reconcile", func() {
		Context("when manifest with ops is created", func() {
			It("handles an error when generating variables", func() {
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						instance.DeepCopyInto(object.(*bdv1.BOSHDeployment))
					case *esv1.ExtendedSecret:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}

					return nil
				})
				client.CreateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *esv1.ExtendedSecret:
						return errors.New("fake-error")
					}
					return nil
				})

				By("From ops applied state to variable interpolated state")
				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to generate variables"))

			})

			It("creates generated variables and update instance state to variable generated state successfully", func() {
				client.UpdateCalls(func(context context.Context, object runtime.Object) error {
					switch object.(type) {
					case *bdv1.BOSHDeployment:
						object.(*bdv1.BOSHDeployment).DeepCopyInto(instance)
					}
					return nil
				})

				By("From ops applied state to variable interpolated state")
				result, err := reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				newInstance := &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, newInstance)
				Expect(err).ToNot(HaveOccurred())
				Expect(newInstance.Status.State).To(Equal(cfd.VariableGeneratedState))
			})
		})
	})
})
