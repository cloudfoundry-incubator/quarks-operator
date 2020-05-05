package quarkssecret_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	certv1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	certv1clientfakes "k8s.io/client-go/kubernetes/typed/certificates/v1beta1/fake"
	ktesting "k8s.io/client-go/testing"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	qsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/client/clientset/versioned/scheme"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/fakes"
	escontroller "code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/quarkssecret"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("ReconcileCertificateSigningRequest", func() {
	var (
		manager          *cfakes.FakeManager
		reconciler       reconcile.Reconciler
		request          reconcile.Request
		ctx              context.Context
		log              *zap.SugaredLogger
		config           *cfcfg.Config
		client           *cfakes.FakeClient
		certClient       *certv1clientfakes.FakeCertificatesV1beta1
		csr              *certv1.CertificateSigningRequest
		privateKeySecret *corev1.Secret
		qsec             *qsv1a1.QuarksSecret
		setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error = func(owner, object metav1.Object, scheme *runtime.Scheme) error { return nil }
	)

	BeforeEach(func() {
		csr = &certv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: "default",
				Annotations: map[string]string{
					qsv1a1.AnnotationCertSecretName: "fake-cert",
					qsv1a1.AnnotationQSecNamespace:  "fake-namespace",
					qsv1a1.AnnotationQSecName:       "fake-name",
				},
			},
			Spec: certv1.CertificateSigningRequestSpec{
				Request: []byte("fake-certificate-signing-request"),
			},
		}
		privateKeySecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      names.CsrPrivateKeySecretName(csr.Name),
				Namespace: "fake-namespace",
			},
			Data: map[string][]byte{
				"private_key": []byte("fake-private-key"),
				"is_ca":       []byte("false"),
			},
		}

		qsec = &qsv1a1.QuarksSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fake-name",
				Namespace: "fake-namespace",
			},
		}

		controllers.AddToScheme(scheme.Scheme)
		manager = &cfakes.FakeManager{}
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}
		config = &cfcfg.Config{CtxTimeOut: 10 * time.Second}
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		client = &cfakes.FakeClient{}
		client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
			switch object := object.(type) {
			case *certv1.CertificateSigningRequest:
				csr.DeepCopyInto(object)
				return nil
			case *corev1.Secret:
				if nn.Name == names.CsrPrivateKeySecretName(csr.Name) {
					privateKeySecret.DeepCopyInto(object)
					return nil
				}
			case *qsv1a1.QuarksSecret:
				if nn.Name == "fake-name" {
					qsec.DeepCopyInto(object)
					return nil
				}
			}
			return apierrors.NewNotFound(schema.GroupResource{}, "not found")
		})

		manager.GetClientReturns(client)

		certClient = &certv1clientfakes.FakeCertificatesV1beta1{
			Fake: &ktesting.Fake{},
		}
	})

	JustBeforeEach(func() {
		reconciler = escontroller.NewCertificateSigningRequestReconciler(ctx, config, manager, certClient, setReferenceFunc)
	})

	Context("when reconciling pending CSR", func() {
		BeforeEach(func() {
			certClient.AddReactor("get", "certificatesigningrequests", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				if action, ok := action.(ktesting.GetActionImpl); ok {
					Expect(action.Name).To(Equal(csr.Name))
					return true, csr, nil
				}
				return true, &certv1.CertificateSigningRequest{}, apierrors.NewNotFound(schema.GroupResource{}, "not found")
			})
			certClient.AddReactor("update", "certificatesigningrequests", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				if action, ok := action.(ktesting.UpdateActionImpl); ok {
					switch object := action.Object.(type) {
					case *certv1.CertificateSigningRequest:
						Expect(object.Status.Conditions).To(ContainElement(certv1.CertificateSigningRequestCondition{
							Type:    certv1.CertificateApproved,
							Reason:  "AutoApproved",
							Message: "This CSR was approved by csr-controller",
						}))
						return true, csr, nil
					}
				}

				return true, &certv1.CertificateSigningRequest{}, apierrors.NewBadRequest("fake-error")
			})
		})
		It("approves CSR", func() {
			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
		})

		It("skips if the resource was not found", func() {
			client.GetReturns(apierrors.NewNotFound(schema.GroupResource{}, "not found"))

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
		})

		It("handles an error when getting certificatesigningrequest", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object.(type) {
				case *certv1.CertificateSigningRequest:
					return apierrors.NewBadRequest("fake-error")
				}
				return apierrors.NewNotFound(schema.GroupResource{}, "not found")
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Error reading CSR"))
		})

		It("handles an error when getting pending certificatesigningrequest", func() {
			certClient.PrependReactor("get", "certificatesigningrequests", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				if action, ok := action.(ktesting.GetActionImpl); ok {
					Expect(action.Name).To(Equal(csr.Name))
					return true, &certv1.CertificateSigningRequest{}, apierrors.NewNotFound(schema.GroupResource{}, "not found")
				}
				return true, &certv1.CertificateSigningRequest{}, nil
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not get CSR"))
		})

		It("skips if pending certificatesigningrequest has been approved", func() {
			certClient.PrependReactor("get", "certificatesigningrequests", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				if action, ok := action.(ktesting.GetActionImpl); ok {
					Expect(action.Name).To(Equal(csr.Name))
					csr.Status.Conditions = append(csr.Status.Conditions, certv1.CertificateSigningRequestCondition{
						Type:    certv1.CertificateApproved,
						Reason:  "AutoApproved",
						Message: "This CSR was approved by csr-controller",
					})
					return true, csr, nil
				}
				return true, &certv1.CertificateSigningRequest{}, apierrors.NewNotFound(schema.GroupResource{}, "not found")
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
		})

		It("handles an error when updating approval of certificatesigningrequest", func() {
			certClient.PrependReactor("update", "certificatesigningrequests", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &certv1.CertificateSigningRequest{}, apierrors.NewBadRequest("fake-error")
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not update approval of CSR"))
		})
	})

	Context("when reconciling approved CSR", func() {
		BeforeEach(func() {
			csr.Status = certv1.CertificateSigningRequestStatus{
				Conditions: []certv1.CertificateSigningRequestCondition{
					{
						Type:    certv1.CertificateApproved,
						Reason:  "AutoApproved",
						Message: "This CSR was approved by csr-controller",
					},
				},
				Certificate: []byte("fake-issued-certificate"),
			}

			client.ListCalls(func(context context.Context, object runtime.Object, _ ...crc.ListOption) error {
				switch object := object.(type) {
				case *corev1.SecretList:
					object.Items = append(object.Items, corev1.Secret{
						Type: "kubernetes.io/service-account-token",
						Data: map[string][]byte{
							"ca.crt": []byte("foo"),
						},
					})
				}
				return nil
			})

			client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
				switch object := object.(type) {
				case *corev1.Secret:
					Expect(object.Name).To(Equal("fake-cert"))
					Expect(object.Data).To(HaveKeyWithValue("ca", []byte("foo")))
					Expect(object.Data).To(HaveKeyWithValue("certificate", csr.Status.Certificate))
					Expect(object.Data).To(HaveKeyWithValue("private_key", privateKeySecret.Data["private_key"]))
					Expect(object.Data).To(HaveKeyWithValue("is_ca", privateKeySecret.Data["is_ca"]))
					return nil
				}

				return apierrors.NewBadRequest("fake-error")
			})

			client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOption) error {
				switch object := object.(type) {
				case *corev1.Secret:
					Expect(object.GetName()).To(Equal(names.CsrPrivateKeySecretName(csr.Name)))
					return nil
				}
				return nil
			})
		})

		It("creates cert secret and cleans up resources", func() {
			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
			Expect(client.GetCallCount()).To(Equal(4))
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(client.DeleteCallCount()).To(Equal(2))
		})

		It("Skips reconcile when getting nil annotations", func() {
			csr.Annotations = nil

			_, err := reconciler.Reconcile(request)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

		It("Skips reconcile when getting cert secret name", func() {
			csr.Annotations = map[string]string{}

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

		It("Skips reconcile when getting quarksSecret's namespace", func() {
			csr.Annotations = map[string]string{
				qsv1a1.AnnotationCertSecretName: "fake-cert",
			}

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(reconcile.Result{}).To(Equal(result))
			Expect(client.GetCallCount()).To(Equal(1))
			Expect(client.CreateCallCount()).To(Equal(0))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

		It("handles an error when getting private key secret", func() {
			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				switch object := object.(type) {
				case *certv1.CertificateSigningRequest:
					csr.DeepCopyInto(object)
					return nil
				case *corev1.Secret:
					if nn.Name == names.CsrPrivateKeySecretName(csr.Name) {
						return apierrors.NewNotFound(schema.GroupResource{}, "not found")
					}
					return nil
				}
				return apierrors.NewNotFound(schema.GroupResource{}, "not found")
			})

			result, err := reconciler.Reconcile(request)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())
		})

		It("handles an error when creating certificate secret", func() {
			client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
				return apierrors.NewBadRequest("fake-error")
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not create or update secret"))
			Expect(client.GetCallCount()).To(Equal(4))
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(client.DeleteCallCount()).To(Equal(0))
		})

		It("handles an error when deleting private key secret", func() {
			client.DeleteCalls(func(context context.Context, object runtime.Object, opts ...crc.DeleteOption) error {
				return apierrors.NewBadRequest("fake-error")
			})

			_, err := reconciler.Reconcile(request)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("could not delete secret"))
			Expect(client.GetCallCount()).To(Equal(4))
			Expect(client.CreateCallCount()).To(Equal(1))
			Expect(client.DeleteCallCount()).To(Equal(1))
		})
	})
})
