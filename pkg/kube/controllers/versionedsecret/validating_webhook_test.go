package versionedsecret_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/versionedsecret"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	vss "code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("When the webhook handles update request of a secret", func() {
	var (
		log            *zap.SugaredLogger
		ctx            context.Context
		decoder        *admission.Decoder
		validator      admission.Handler
		secretBytes    []byte
		oldSecretBytes []byte
		validateSecret func() admission.Response
		secret         corev1.Secret
	)

	BeforeEach(func() {
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		secret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mysecret",
				Labels: map[string]string{
					vss.LabelSecretKind: "versionedSecret",
				},
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}
	})

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		decoder, _ = admission.NewDecoder(scheme)
		validator = versionedsecret.NewValidationHandler(log)
		validator.(admission.DecoderInjector).InjectDecoder(decoder)

		validateSecret = func() admission.Response {
			response := validator.Handle(ctx, admission.Request{
				AdmissionRequest: v1beta1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: secretBytes,
					},
					OldObject: runtime.RawExtension{
						Raw: oldSecretBytes,
					},
				},
			})
			return response
		}
	})

	Context("which is not a versioned type", func() {
		BeforeEach(func() {
			secret.SetLabels(map[string]string{})
			secretBytes, _ = json.Marshal(secret)
			secret.Data["new"] = []byte("value")
			oldSecretBytes, _ = json.Marshal(secret)
		})

		It("should allow", func() {
			response := validateSecret()
			Expect(response.AdmissionResponse.Allowed).To(BeTrue())
		})
	})

	Context("which is a versioned type", func() {
		Context("when updating data", func() {
			BeforeEach(func() {
				secretBytes, _ = json.Marshal(secret)
				secret.Data["new"] = []byte("value")
				oldSecretBytes, _ = json.Marshal(secret)
			})

			It("should not allow", func() {
				response := validateSecret()
				Expect(response.AdmissionResponse.Allowed).To(BeFalse())
				Expect(response.AdmissionResponse.Result.Message).To(Equal("Denying update to versioned secret 'mysecret' as it is immutable."))
			})
		})

		Context("when updating meta", func() {
			BeforeEach(func() {
				secretBytes, _ = json.Marshal(secret)
				secret.SetLabels(map[string]string{"foo": "bar"})
				oldSecretBytes, _ = json.Marshal(secret)
			})

			It("should allow", func() {
				response := validateSecret()
				Expect(response.AdmissionResponse.Allowed).To(BeTrue())
			})
		})
	})
})
