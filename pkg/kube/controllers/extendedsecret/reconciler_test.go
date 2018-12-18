package extendedsecret_test

import (
	"context"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	esapi "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned/scheme"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	escontroller "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"

	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ReconcileExtendedSecret", func() {
	var (
		manager    *cfakes.FakeManager
		reconciler reconcile.Reconciler
		request    reconcile.Request
		log        *zap.SugaredLogger
		client     *cfakes.FakeClient
		generator  credsgen.Generator
		es         *esapi.ExtendedSecret
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()
		es = &esapi.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: esapi.ExtendedSecretSpec{
				Type: "password",
			},
		}
		generator = inmemorygenerator.NewInMemoryGenerator(log)
		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			es.DeepCopyInto(object.(*esapi.ExtendedSecret))
			return nil
		})
		manager.GetClientReturns(client)
	})

	JustBeforeEach(func() {
		reconciler = escontroller.NewReconciler(log, manager, generator)
	})

	Context("if the resource can not be resolved", func() {
		It("skips if the resource was not found", func() {
			client.GetReturns(errors.NewNotFound(schema.GroupResource{}, "not found is requeued"))

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})

	Context("if the resource is invalid", func() {
		It("returns an error", func() {
			es.Spec.Type = "foo"

			result, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Invalid type"))
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})

	Context("when generating passwords", func() {
		It("generates passwords", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object) error {
				Expect(object.(*corev1.Secret).StringData["password"]).To(MatchRegexp("^\\w{64}$"))
				Expect(object.(*corev1.Secret).GetName()).To(Equal("es-secret-foo"))
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})

	Context("when generating RSA keys", func() {
		BeforeEach(func() {
			es.Spec.Type = "rsa"
		})

		It("generates RSA keys", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object) error {
				Expect(object.(*corev1.Secret).Data["RSAPrivateKey"]).To(ContainSubstring("RSA PRIVATE KEY"))
				Expect(object.(*corev1.Secret).Data["RSAPublicKey"]).To(ContainSubstring("PUBLIC KEY"))
				Expect(object.(*corev1.Secret).GetName()).To(Equal("es-secret-foo"))
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})

	Context("when generating SSH keys", func() {
		BeforeEach(func() {
			es.Spec.Type = "ssh"
		})

		It("generates SSH keys", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object) error {
				secret := object.(*corev1.Secret)
				Expect(secret.Data["SSHPrivateKey"]).To(ContainSubstring("RSA PRIVATE KEY"))
				Expect(secret.Data["SSHPublicKey"]).To(MatchRegexp("ssh-rsa\\s.+"))
				Expect(secret.Data["SSHFingerprint"]).To(MatchRegexp("([0-9a-f]{2}:){15}[0-9a-f]{2}"))
				Expect(secret.GetName()).To(Equal("es-secret-foo"))
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})
})
