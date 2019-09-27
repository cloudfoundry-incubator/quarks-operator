package extendedsecret_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	generatorfakes "code.cloudfoundry.org/cf-operator/pkg/credsgen/fakes"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned/scheme"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	escontroller "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/extendedsecret"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	cfcfg "code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
)

var _ = Describe("ReconcileExtendedSecret", func() {
	var (
		manager          *cfakes.FakeManager
		reconciler       reconcile.Reconciler
		request          reconcile.Request
		ctx              context.Context
		log              *zap.SugaredLogger
		config           *cfcfg.Config
		client           *cfakes.FakeClient
		generator        *generatorfakes.FakeGenerator
		es               *esv1.ExtendedSecret
		setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error = func(owner, object metav1.Object, scheme *runtime.Scheme) error { return nil }
	)

	BeforeEach(func() {
		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		es = &esv1.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
			},
			Spec: esv1.ExtendedSecretSpec{
				Type:       "password",
				SecretName: "generated-secret",
			},
		}
		generator = &generatorfakes.FakeGenerator{}
		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *esv1.ExtendedSecret:
				es.DeepCopyInto(object)
			case *corev1.Secret:
				return errors.NewNotFound(schema.GroupResource{}, "not found")
			}
			return nil
		})
		client.StatusCalls(func() crc.StatusWriter { return &cfakes.FakeStatusWriter{} })
		manager.GetClientReturns(client)
	})

	JustBeforeEach(func() {
		reconciler = escontroller.NewExtendedSecretReconciler(ctx, config, manager, generator, setReferenceFunc)
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
			client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
				secret := object.(*corev1.Secret)
				Expect(secret.StringData["password"]).To(Equal("securepassword"))
				Expect(secret.GetName()).To(Equal("generated-secret"))
				Expect(secret.GetLabels()).To(HaveKeyWithValue(esv1.LabelKind, esv1.GeneratedSecretKind))
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
			client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
				secret := object.(*corev1.Secret)
				Expect(secret.StringData["private_key"]).To(Equal("private"))
				Expect(secret.StringData["public_key"]).To(Equal("public"))
				Expect(secret.GetName()).To(Equal("generated-secret"))
				Expect(secret.GetLabels()).To(HaveKeyWithValue(esv1.LabelKind, esv1.GeneratedSecretKind))
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
			client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
				secret := object.(*corev1.Secret)
				Expect(secret.StringData["private_key"]).To(Equal("private"))
				Expect(secret.StringData["public_key"]).To(Equal("public"))
				Expect(secret.StringData["public_key_fingerprint"]).To(Equal("fingerprint"))
				Expect(secret.GetName()).To(Equal("generated-secret"))
				Expect(secret.GetLabels()).To(HaveKeyWithValue(esv1.LabelKind, esv1.GeneratedSecretKind))
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
			es.Spec.Request.CertificateRequest.CARef = esv1.SecretReference{Name: "mysecret", Key: "ca"}
			es.Spec.Request.CertificateRequest.CAKeyRef = esv1.SecretReference{Name: "mysecret", Key: "key"}
			es.Spec.Request.CertificateRequest.CommonName = "foo.com"
			es.Spec.Request.CertificateRequest.AlternativeNames = []string{"bar.com", "baz.com"}
		})

		Context("if the CA is not ready", func() {
			It("requeues generation", func() {
				result, err := reconciler.Reconcile(request)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(0))
				Expect(reconcile.Result{RequeueAfter: time.Second * 5}).To(Equal(result))
			})
		})

		Context("if the CA is ready", func() {
			BeforeEach(func() {
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
					switch object := object.(type) {
					case *esv1.ExtendedSecret:
						es.DeepCopyInto(object)
					case *corev1.Secret:
						if nn.Name == "mysecret" {
							ca.DeepCopyInto(object)
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
				client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
					secret := object.(*corev1.Secret)
					Expect(secret.StringData["certificate"]).To(Equal("the_cert"))
					Expect(secret.StringData["private_key"]).To(Equal("private_key"))
					Expect(secret.StringData["ca"]).To(Equal("theca"))
					Expect(secret.GetLabels()).To(HaveKeyWithValue(esv1.LabelKind, esv1.GeneratedSecretKind))
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

	Context("when secret is set manually", func() {
		var (
			password string
			secret   *corev1.Secret
		)

		BeforeEach(func() {
			es.Spec.Type = "password"
			es.Spec.SecretName = "mysecret"

			password = "new-generated-password"
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mysecret",
					Namespace: "default",
				},
				StringData: map[string]string{
					"password": "securepassword",
				},
			}

			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *esv1.ExtendedSecret:
					es.DeepCopyInto(object)
				case *corev1.Secret:
					if nn.Name == "mysecret" {
						secret.DeepCopyInto(object)
					} else {
						return errors.NewNotFound(schema.GroupResource{}, "not found is requeued")
					}
				}
				return nil
			})

			generator.GeneratePasswordReturns(password)
		})

		It("Skips generation of a secret when existing secret has not `generated` label", func() {
			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(client.UpdateCallCount()).To(Equal(0))
			Expect(reconcile.Result{}).To(Equal(result))
		})

		It("Skips generation of a secret when extendedSecret's `generated` status is true", func() {
			secret.Labels = map[string]string{
				esv1.LabelKind: esv1.GeneratedSecretKind,
			}
			es.Status.Generated = true

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(client.UpdateCallCount()).To(Equal(0))
			Expect(reconcile.Result{}).To(Equal(result))
		})

		It("Regenerate generation of a secret when existing secret has `generated` label", func() {
			secret.Labels = map[string]string{
				esv1.LabelKind: esv1.GeneratedSecretKind,
			}

			client.UpdateCalls(func(context context.Context, object runtime.Object, _ ...crc.UpdateOption) error {
				switch object := object.(type) {
				case *esv1.ExtendedSecret:
					Expect(object.Status.Generated).To(Equal(true))
				case *corev1.Secret:
					Expect(object.StringData["password"]).To(Equal(password))
					Expect(object.GetName()).To(Equal("mysecret"))
					Expect(object.GetLabels()).To(HaveKeyWithValue(esv1.LabelKind, esv1.GeneratedSecretKind))
				}
				return nil
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(client.UpdateCallCount()).To(Equal(2))
			Expect(reconcile.Result{}).To(Equal(result))

		})
	})
})
