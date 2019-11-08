package boshdeployment_test

import (
	"context"
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

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter/fakes"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfd "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/boshdeployment"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileGeneratedVariable", func() {
	var (
		manager               *cfakes.FakeManager
		reconciler            reconcile.Reconciler
		recorder              *record.FakeRecorder
		request               reconcile.Request
		ctx                   context.Context
		log                   *zap.SugaredLogger
		config                *cfcfg.Config
		client                *cfakes.FakeClient
		kubeConverter         *fakes.FakeKubeConverter
		manifestWithOpsSecret *corev1.Secret
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		recorder = record.NewFakeRecorder(20)
		manager = &cfakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)
		manager.GetEventRecorderForReturns(recorder)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo-with-ops", Namespace: "default"}}

		manifestWithOpsSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo-with-ops",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"manifest.yaml": []byte(`---
name: fake-manifest
releases:
- name: bar
  url: docker.io/cfcontainerization
  version: 1.0
  stemcell:
    os: opensuse
    version: 42.3
instance_groups:
- name: fakepod
  jobs:
  - name: foo
    release: bar
    properties:
      password: ((foo_password))
      quarks:
        ports:
        - name: foo
          protocol: TCP
          internal: 8080
variables:
- name: foo_password
  type: password
`),
			},
		}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		ctx = ctxlog.NewContextWithRecorder(ctx, "TestRecorder", recorder)

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *corev1.Secret:
				if nn.Name == "foo-with-ops" {
					manifestWithOpsSecret.DeepCopyInto(object)
				}
			}

			return nil
		})

		manager.GetClientReturns(client)

		kubeConverter = &fakes.FakeKubeConverter{}
		kubeConverter.VariablesReturns([]qsv1a1.QuarksSecret{}, nil)
	})

	JustBeforeEach(func() {
		reconciler = cfd.NewGeneratedVariableReconciler(ctx, config, manager, controllerutil.SetControllerReference, kubeConverter)
	})

	Describe("Reconcile", func() {
		Context("when manifest with ops is created", func() {
			It("handles an error when generating variables", func() {
				kubeConverter.VariablesReturns([]qsv1a1.QuarksSecret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "fake-variable",
						},
					},
				}, nil)

				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *corev1.Secret:
						if nn.Name == "foo-with-ops" {
							manifestWithOpsSecret.DeepCopyInto(object)
						}
					case *qsv1a1.QuarksSecret:
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
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

				By("From ops applied state to variable interpolated state")
				_, err := reconciler.Reconcile(request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to generate variables"))

			})

			It("creates generated variables", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				newInstance := &bdv1.BOSHDeployment{}
				err = client.Get(context.Background(), types.NamespacedName{Name: "foo", Namespace: "default"}, newInstance)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
