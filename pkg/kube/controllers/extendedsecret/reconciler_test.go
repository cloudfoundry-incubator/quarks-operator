package extendedsecret_test

import (
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	generatorfakes "code.cloudfoundry.org/cf-operator/pkg/credsgen/fakes"
	esapi "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned/scheme"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	escontroller "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"

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
		manager          *cfakes.FakeManager
		reconciler       reconcile.Reconciler
		request          reconcile.Request
		log              *zap.SugaredLogger
		ctrsConfig       *context.Config
		client           *cfakes.FakeClient
		generator        *generatorfakes.FakeGenerator
		es               *esapi.ExtendedSecret
		setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error = func(owner, object metav1.Object, scheme *runtime.Scheme) error { return nil }
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		core, _ := observer.New(zapcore.InfoLevel)
		log = zap.New(core).Sugar()
		ctrsConfig = &context.Config{ //Set the context to be TODO
			CtxTimeOut: 10 * time.Second,
			CtxType:    context.NewContext(),
		}
		es = &esapi.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: esapi.ExtendedSecretSpec{
				Type:       "password",
				SecretName: "generated-secret",
			},
		}
		generator = &generatorfakes.FakeGenerator{}
		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object.(type) {
			case *esapi.ExtendedSecret:
				es.DeepCopyInto(object.(*esapi.ExtendedSecret))
			case *corev1.Secret:
				return errors.NewNotFound(schema.GroupResource{}, "not found")
			}
			return nil
		})
		manager.GetClientReturns(client)
	})

	JustBeforeEach(func() {
		reconciler = escontroller.NewReconciler(log, ctrsConfig, manager, generator, setReferenceFunc)
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
			Expect(err.Error()).To(ContainSubstring("invalid type"))
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})

	Context("when generating passwords", func() {
		BeforeEach(func() {
			generator.GeneratePasswordReturns("securepassword")
		})

		It("skips reconciling if the secret was already generated", func() {
			client.GetReturns(nil)

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(reconcile.Result{}).To(Equal(result))
		})

		It("generates passwords", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object) error {
				Expect(object.(*corev1.Secret).StringData["password"]).To(Equal("securepassword"))
				Expect(object.(*corev1.Secret).GetName()).To(Equal("generated-secret"))
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

			generator.GenerateRSAKeyReturns(credsgen.RSAKey{PrivateKey: []byte("private"), PublicKey: []byte("public")}, nil)
		})

		It("generates RSA keys", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object) error {
				Expect(object.(*corev1.Secret).Data["private_key"]).To(Equal([]byte("private")))
				Expect(object.(*corev1.Secret).Data["public_key"]).To(Equal([]byte("public")))
				Expect(object.(*corev1.Secret).GetName()).To(Equal("generated-secret"))
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

			generator.GenerateSSHKeyReturns(credsgen.SSHKey{
				PrivateKey:  []byte("private"),
				PublicKey:   []byte("public"),
				Fingerprint: "fingerprint",
			}, nil)
		})

		It("generates SSH keys", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object) error {
				secret := object.(*corev1.Secret)
				Expect(secret.Data["private_key"]).To(Equal([]byte("private")))
				Expect(secret.Data["public_key"]).To(Equal([]byte("public")))
				Expect(secret.Data["public_key_fingerprint"]).To(Equal([]byte("fingerprint")))
				Expect(object.(*corev1.Secret).GetName()).To(Equal("generated-secret"))
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})

	Context("when generating certificates", func() {
		BeforeEach(func() {
			es.Spec.Type = "certificate"
			es.Spec.Request.CertificateRequest.IsCA = false
			es.Spec.Request.CertificateRequest.CARef = esapi.SecretReference{Name: "mysecret", Key: "ca"}
			es.Spec.Request.CertificateRequest.CAKeyRef = esapi.SecretReference{Name: "mysecret", Key: "key"}
			es.Spec.Request.CertificateRequest.CommonName = "foo.com"
			es.Spec.Request.CertificateRequest.AlternativeNames = []string{"bar.com", "baz.com"}

			ca := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"ca":  []byte("theca"),
					"key": []byte("the_private_key"),
				},
			}

			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object.(type) {
				case *esapi.ExtendedSecret:
					es.DeepCopyInto(object.(*esapi.ExtendedSecret))
				case *corev1.Secret:
					if nn.Name == "mysecret" {
						ca.DeepCopyInto(object.(*corev1.Secret))
					} else {
						return errors.NewNotFound(schema.GroupResource{}, "not found is requeued")
					}
				}
				return nil
			})
		})

		It("triggers generation of a secret", func() {
			generator.GenerateCertificateCalls(func(name string, request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
				Expect(request.CA.Certificate).To(Equal([]byte("theca")))
				Expect(request.CA.PrivateKey).To(Equal([]byte("the_private_key")))

				return credsgen.Certificate{Certificate: []byte("the_cert"), PrivateKey: []byte("private_key"), IsCA: false}, nil
			})
			client.CreateCalls(func(context context.Context, object runtime.Object) error {
				secret := object.(*corev1.Secret)
				Expect(secret.Data["certificate"]).To(Equal([]byte("the_cert")))
				Expect(secret.Data["private_key"]).To(Equal([]byte("private_key")))
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(reconcile.Result{}).To(Equal(result))
		})

		It("considers generation parameters", func() {
			generator.GenerateCertificateCalls(func(name string, request credsgen.CertificateGenerationRequest) (credsgen.Certificate, error) {
				Expect(request.IsCA).To(BeFalse())
				Expect(request.CommonName).To(Equal("foo.com"))
				Expect(request.AlternativeNames).To(Equal([]string{"bar.com", "baz.com"}))
				return credsgen.Certificate{Certificate: []byte("the_cert"), PrivateKey: []byte("private_key"), IsCA: false}, nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(reconcile.Result{}).To(Equal(result))
		})
	})
})
