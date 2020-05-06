package boshdeployment_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/boshdeployment"
	"code.cloudfoundry.org/quarks-operator/testing"
	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("When the validating webhook handles a manifest", func() {
	var (
		log                    *zap.SugaredLogger
		ctx                    context.Context
		env                    testing.Catalog
		client                 client.Client
		decoder                *admission.Decoder
		manifest               *manifest.Manifest
		validator              admission.Handler
		boshDeploymentBytes    []byte
		validateBoshDeployment func() admission.Response
	)

	BeforeEach(func() {
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		boshDeployment := bdv1.BOSHDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deployment",
				Namespace: "default",
			},
			Spec: bdv1.BOSHDeploymentSpec{
				Manifest: bdv1.ResourceReference{
					Type: bdv1.ConfigMapReference,
					Name: "base-manifest",
				},
			},
		}
		boshDeploymentBytes, _ = json.Marshal(boshDeployment)
		manifest, _ = env.BOSHManifestWithZeroInstances()
	})

	JustBeforeEach(func() {
		manifestBytes, _ := manifest.Marshal()
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(bdv1.AddToScheme(scheme)).To(Succeed())
		client = fake.NewFakeClientWithScheme(scheme, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "base-manifest",
				Namespace: "default",
			},
			Data: map[string]string{
				bdv1.ManifestSpecName: string(manifestBytes),
			},
		})
		decoder, _ = admission.NewDecoder(scheme)
		validator = boshdeployment.NewValidator(log, &cfcfg.Config{CtxTimeOut: 10 * time.Second})
		validator.(inject.Client).InjectClient(client)
		validator.(admission.DecoderInjector).InjectDecoder(decoder)

		validateBoshDeployment = func() admission.Response {
			response := validator.Handle(ctx, admission.Request{
				AdmissionRequest: v1beta1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: boshDeploymentBytes,
					},
				},
			})

			return response
		}
	})

	Context("with an invalid canary_watch_time", func() {
		BeforeEach(func() {
			manifest.Update.CanaryWatchTime = "notANumber"
		})

		It("the manifest is rejected", func() {
			response := validateBoshDeployment()
			Expect(response.AdmissionResponse.Allowed).To(BeFalse())
			Expect(response.AdmissionResponse.Result.Message).To(ContainSubstring("invalid canary_watch_time"))
		})
	})

	Context("with a canary_watch_time range", func() {
		BeforeEach(func() {
			manifest.Update.CanaryWatchTime = "30000 - 1200000"
		})

		It("the manifest is accepted", func() {
			response := validateBoshDeployment()
			Expect(response.AdmissionResponse.Allowed).To(BeTrue())
		})
	})

	Context("with an absolute canary_watch_time", func() {
		BeforeEach(func() {
			manifest.Update.CanaryWatchTime = "30000"
		})

		It("the manifest is accepted", func() {
			response := validateBoshDeployment()
			Expect(response.AdmissionResponse.Allowed).To(BeTrue(), response.Result.String)
		})
	})

	Context("with a canary_watch_time containing measurement", func() {
		BeforeEach(func() {
			manifest.Update.CanaryWatchTime = "30000ms"
		})

		It("the manifest is rejected", func() {
			response := validateBoshDeployment()
			Expect(response.AdmissionResponse.Allowed).To(BeFalse())
		})
	})
})
