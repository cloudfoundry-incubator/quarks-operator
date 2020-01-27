package controllers_test

import (
	"context"
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/spf13/afero"

	admissionregistration "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	gfakes "code.cloudfoundry.org/cf-operator/pkg/credsgen/fakes"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	cfakes "code.cloudfoundry.org/cf-operator/pkg/kube/controllers/fakes"
	"code.cloudfoundry.org/cf-operator/testing"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	cmdhelper "code.cloudfoundry.org/quarks-utils/testing"
)

var _ = Describe("Controllers", func() {
	Describe("AddToScheme", func() {
		It("registers our schemes with the operator", func() {
			scheme := scheme.Scheme
			controllers.AddToScheme(scheme)
			kinds := []string{}
			for k := range scheme.AllKnownTypes() {
				kinds = append(kinds, k.Kind)
			}
			Expect(kinds).To(ContainElement("BOSHDeployment"))
			Expect(kinds).To(ContainElement("QuarksSecret"))
			Expect(kinds).To(ContainElement("QuarksStatefulSet"))
		})
	})

	// "AddToManager" tested via integration tests

	Describe("AddHooks", func() {
		var (
			manager   *cfakes.FakeManager
			client    *cfakes.FakeClient
			ctx       context.Context
			config    *config.Config
			generator *gfakes.FakeGenerator
			env       testing.Catalog
		)

		BeforeEach(func() {

			client = &cfakes.FakeClient{}
			restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{})
			restMapper.Add(schema.GroupVersionKind{Group: "", Kind: "Pod", Version: "v1"}, meta.RESTScopeNamespace)
			restMapper.Add(schema.GroupVersionKind{Group: "quarks.cloudfoundry.org", Kind: "BOSHDeployment", Version: "v1alpha1"}, meta.RESTScopeNamespace)

			manager = &cfakes.FakeManager{}

			controllers.AddToScheme(scheme.Scheme)

			manager.GetSchemeReturns(scheme.Scheme)

			manager.GetClientReturns(client)
			//manager.GetRecorderReturns(recorder)
			manager.GetRESTMapperReturns(restMapper)

			manager.GetWebhookServerReturns(&webhook.Server{})

			generator = &gfakes.FakeGenerator{}
			generator.GenerateCertificateReturns(credsgen.Certificate{Certificate: []byte("thecert")}, nil)

			config = env.DefaultConfig()
			ctx = cmdhelper.NewContext()
		})

		It("sets the operator namespace label", func() {
			client.UpdateCalls(func(_ context.Context, object runtime.Object, _ ...crc.UpdateOption) error {
				ns := object.(*unstructured.Unstructured)
				labels := ns.GetLabels()

				Expect(labels["cf-operator-ns"]).To(Equal(config.Namespace))

				return nil
			})

			client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
				kind := object.GetObjectKind()
				if kind.GroupVersionKind().Kind == "Secret" {
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				}
				return nil
			})

			err := controllers.AddHooks(ctx, config, manager, generator)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("if there is no cert secret yet", func() {
			It("generates and persists the certificates on disk and in a secret", func() {
				file := "/tmp/cf-operator-hook-" + config.OperatorNamespace + "/tls.key"
				Expect(afero.Exists(config.Fs, file)).To(BeFalse())

				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					kind := object.GetObjectKind()
					if kind.GroupVersionKind().Kind == "Secret" {
						return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
					}
					return nil
				})

				err := controllers.AddHooks(ctx, config, manager, generator)
				Expect(err).ToNot(HaveOccurred())

				Expect(afero.Exists(config.Fs, file)).To(BeTrue())
				Expect(generator.GenerateCertificateCallCount()).To(Equal(2)) // Generate CA and certificate
				Expect(client.CreateCallCount()).To(Equal(3))                 // Persist secret and the 2 webhook configs (Validating and Mutating)
			})
		})

		Context("if there is a persisted cert secret already", func() {
			BeforeEach(func() {
				secret := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"metadata": map[string]interface{}{
							"name":      "cf-operator-webhook-server-cert",
							"namespace": config.OperatorNamespace,
						},
						"data": map[string]interface{}{
							"certificate":    base64.StdEncoding.EncodeToString([]byte("the-cert")),
							"private_key":    base64.StdEncoding.EncodeToString([]byte("the-key")),
							"ca_certificate": base64.StdEncoding.EncodeToString([]byte("the-ca-cert")),
							"ca_private_key": base64.StdEncoding.EncodeToString([]byte("the-ca-key")),
						},
					},
				}
				client.GetCalls(func(context context.Context, nn types.NamespacedName, object runtime.Object) error {
					switch object := object.(type) {
					case *unstructured.Unstructured:
						secret.DeepCopyInto(object)
						return nil
					}
					return apierrors.NewNotFound(schema.GroupResource{}, nn.Name)
				})
			})

			It("does not overwrite the existing secret", func() {
				err := controllers.AddHooks(ctx, config, manager, generator)
				Expect(err).ToNot(HaveOccurred())
				Expect(client.CreateCallCount()).To(Equal(2)) // webhook config for Mutation and Validation
			})

			It("generates the webhook configuration", func() {
				client.CreateCalls(func(context context.Context, object runtime.Object, _ ...crc.CreateOption) error {
					// We should be getting 2 Create calls - one for the
					// Validation webhook, one for the Mutating Webhook

					switch config := object.(type) {
					case *admissionregistration.MutatingWebhookConfiguration:
						Expect(config.Name).To(Equal("cf-operator-hook-" + config.Namespace))
						Expect(len(config.Webhooks)).To(Equal(3))

						wh := config.Webhooks[0]
						Expect(wh.Name).To(Equal("mutate-pods.quarks.cloudfoundry.org"))
						Expect(*wh.ClientConfig.URL).To(Equal("https://foo.com:1234/mutate-pods"))
						Expect(wh.ClientConfig.CABundle).To(ContainSubstring("the-ca-cert"))
						Expect(*wh.FailurePolicy).To(Equal(admissionregistration.Fail))
						return nil
					case *admissionregistration.ValidatingWebhookConfiguration:
						Expect(config.Name).To(Equal("cf-operator-hook-" + config.Namespace))
						Expect(len(config.Webhooks)).To(Equal(2))

						wh := config.Webhooks[0]
						Expect(wh.Name).To(Equal("validate-boshdeployment.quarks.cloudfoundry.org"))
						Expect(*wh.ClientConfig.URL).To(Equal("https://foo.com:1234/validate-boshdeployment"))
						Expect(wh.ClientConfig.CABundle).To(ContainSubstring("the-ca-cert"))
						Expect(*wh.FailurePolicy).To(Equal(admissionregistration.Fail))
						return nil
					default:
						return errors.New("unexpected type")
					}
				})
				err := controllers.AddHooks(ctx, config, manager, generator)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
