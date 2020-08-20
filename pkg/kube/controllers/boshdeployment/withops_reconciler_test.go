package boshdeployment_test

import (
	"context"
	"time"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	cfd "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/boshdeployment"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/meltdown"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("ReconcileWithOps", func() {
	var (
		manager        *fakes.FakeManager
		request        reconcile.Request
		reconciler     reconcile.Reconciler
		ctx            context.Context
		logs           *observer.ObservedLogs
		log            *zap.SugaredLogger
		resolver       fakes.FakeInterpolateSecrets
		config         *cfcfg.Config
		client         *fakes.FakeClient
		withOpsSecret  *corev1.Secret
		passwordSecret *corev1.Secret
		boshDeployment *bdv1.BOSHDeployment
	)

	BeforeEach(func() {
		manager = &fakes.FakeManager{}
		manager.GetSchemeReturns(scheme.Scheme)
		resolver = fakes.FakeInterpolateSecrets{}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		logs, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		meltdownTime := metav1.NewTime(metav1.Now().Add(cfd.ReconcileSkipDuration))
		passwordSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "var-password",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"password": []byte("passwordData"),
			},
		}

		withOpsSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fakeWithOpsSecret",
				Namespace: "default",
				Annotations: map[string]string{
					meltdown.AnnotationLastReconcile: meltdownTime.Format(time.RFC3339),
				},
			},
			Data: map[string][]byte{
				"manifest.yaml": []byte(`director_uuid: ((password))
instance_groups:
- name: gora
  instances: 1
variables:
- name: password
  type: password
`),
			},
		}

		boshDeployment = &bdv1.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gora",
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
				},
			},
		}

		client = &fakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *bdv1.BOSHDeployment:
				boshDeployment.DeepCopyInto(object)
			case *corev1.Secret:
				if nn.Name == withOpsSecret.Name {
					withOpsSecret.DeepCopyInto(object)
				}
				if nn.Name == passwordSecret.Name {
					passwordSecret.DeepCopyInto(object)
				}
			}
			return nil
		})

		manager.GetClientReturns(client)

		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "fakeWithOpsSecret", Namespace: "default"}}
	})

	JustBeforeEach(func() {
		reconciler = cfd.NewWithOpsReconciler(
			ctx, config, manager,
			&resolver,
			controllerutil.SetControllerReference,
			func(m bdm.Manifest) (boshdns.DomainNameService, error) {
				return boshdns.NewSimpleDomainNameService(), nil
			},
		)
	})

	Context("WithOps secret is recnociled", func() {
		It("should create the desired manifest secret", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
				switch object := object.(type) {
				case *corev1.Secret:
					secret := object
					Expect(secret.Name).To(Equal("desired-manifest-v1"))
					Expect(secret.Labels).To(Equal(map[string]string{
						"quarks.cloudfoundry.org/deployment-name": "gora",
						"quarks.cloudfoundry.org/secret-kind":     "versionedSecret",
						"quarks.cloudfoundry.org/secret-type":     "desired",
						"quarks.cloudfoundry.org/secret-version":  "1",
					}))
				}
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{
				Requeue: false,
			}))
		})

		It("should rqueue after if quarks secret is not found", func() {
			resolver.InterpolateVariableFromSecretsReturns([]byte("test"), errors.New("Expected to find variables: password"))

			result, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(logs.FilterMessageSnippet("Expected to find variables: password").Len()).To(Equal(1))
			Expect(result.RequeueAfter).To(Equal(5 * time.Second))
		})
	})
})
