package boshdeployment_test

import (
	"fmt"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/fakes"
	bdc "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReconcileBoshDeployment", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		resolver   fakes.FakeResolver
		manifest   *bdm.Manifest
		log        *zap.SugaredLogger
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		resolver = fakes.FakeResolver{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		manifest = &bdm.Manifest{
			InstanceGroups: []bdm.InstanceGroup{
				bdm.InstanceGroup{Name: "fakepod"},
			},
		}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()
	})

	JustBeforeEach(func() {
		resolver.ResolveCRDReturns(manifest, nil)
		reconciler = cfd.NewReconciler(log, manager, &resolver, controllerutil.SetControllerReference)
	})

	Describe("Reconcile", func() {
		Context("when the manifest can not be resolved", func() {
			var (
				client *cfakes.FakeClient
			)
			BeforeEach(func() {
				client = &cfakes.FakeClient{}
				manager.GetClientReturns(client)
			})

			It("returns an empty Result when the resource was not found", func() {
				client.GetReturns(errors.NewNotFound(schema.GroupResource{}, "not found is requeued"))

				reconciler.Reconcile(request)
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(reconcile.Result{}).To(Equal(result))
			})

			It("throws an error when the request failed", func() {
				client.GetReturns(errors.NewBadRequest("bad request returns error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bad request returns error"))
			})

			It("handles errors when resolving the CR", func() {
				resolver.ResolveCRDReturns(nil, fmt.Errorf("resolver error"))

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("resolver error"))
			})
		})

		Context("when the manifest can be resolved", func() {
			var (
				client client.Client
			)
			BeforeEach(func() {
				client = fake.NewFakeClient(
					&bdc.BOSHDeployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "foo",
							Namespace: "default",
						},
						Spec: bdc.BOSHDeploymentSpec{},
					},
				)
				manager.GetClientReturns(client)
			})

			Context("With an empty manifest", func() {
				BeforeEach(func() {
					manifest = &bdm.Manifest{}
				})

				It("raises an error if there are no instance groups defined in the manifest", func() {
					_, err := reconciler.Reconcile(request)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("manifest is missing instance groups"))
				})
			})

			It("handles errors when setting the owner reference on the object", func() {
				reconciler = cfd.NewReconciler(log, manager, &resolver, func(owner, object metav1.Object, scheme *runtime.Scheme) error {
					return fmt.Errorf("failed to set reference")
				})

				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to set reference"))
			})
		})
	})
})
