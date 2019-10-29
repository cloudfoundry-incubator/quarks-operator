package statefulset

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"

	cfcfg "code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("When the muatating webhook handles a statefulset", func() {
	var (
		log     *zap.SugaredLogger
		ctx     context.Context
		decoder *admission.Decoder
		mutator admission.Handler
		old     v1beta2.StatefulSet
		new     v1beta2.StatefulSet
		request admission.Request
	)

	BeforeEach(func() {
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)
		old = v1beta2.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-statefulset",
				Namespace: "test",
				Annotations: map[string]string{
					AnnotationCanaryRolloutEnabled: "true",
				},
			},
			Spec: v1beta2.StatefulSetSpec{
				Replicas: pointers.Int32(2),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name: "test-container",
						}},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		decoder, _ = admission.NewDecoder(scheme)
		mutator = NewMutator(log, &cfcfg.Config{CtxTimeOut: 10 * time.Second})
		mutator.(admission.DecoderInjector).InjectDecoder(decoder)
	})

	Context("that is newly created", func() {
		BeforeEach(func() {
			new = old
		})

		It("doesn't fail", func() {

			newRaw, _ := json.Marshal(new)

			response := mutator.Handle(ctx, admission.Request{
				AdmissionRequest: admissionv1beta1.AdmissionRequest{
					Object:    runtime.RawExtension{Raw: newRaw},
					Operation: admissionv1beta1.Create,
				},
			})
			Expect(response.AdmissionResponse.Allowed).To(BeTrue())
			Expect(response.Patches).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/metadata/annotations/quarks.cloudfoundry.org~1canary-rollout", Value: "Pending"},
			))
			Expect(response.Patches).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/updateStrategy/type", Value: "RollingUpdate"},
			))
		})
	})

	Context("with no change in pod template", func() {
		BeforeEach(func() {
			raw, _ := json.Marshal(old)

			request = admission.Request{
				AdmissionRequest: admissionv1beta1.AdmissionRequest{
					OldObject: runtime.RawExtension{Raw: raw},
					Object:    runtime.RawExtension{Raw: raw},
					Operation: admissionv1beta1.Update,
				},
			}
		})

		It("no rollout is triggered", func() {
			response := mutator.Handle(ctx, request)
			Expect(response.AdmissionResponse.Allowed).To(BeTrue())
			Expect(response.Patches).To(BeEmpty())
		})
	})

	Context("when pod template changes", func() {
		BeforeEach(func() {
			old.DeepCopyInto(&new)
			new.Spec.Template.Spec.Containers[0].Name = "changed-name"

			oldRaw, _ := json.Marshal(old)
			newRaw, _ := json.Marshal(new)

			request = admission.Request{
				AdmissionRequest: admissionv1beta1.AdmissionRequest{
					OldObject: runtime.RawExtension{Raw: oldRaw},
					Object:    runtime.RawExtension{Raw: newRaw},
					Operation: admissionv1beta1.Update,
				},
			}
		})

		It("rollout is triggered", func() {
			response := mutator.Handle(ctx, request)
			Expect(response.Patches).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/metadata/annotations/quarks.cloudfoundry.org~1canary-rollout", Value: "Pending"},
			))
			Expect(response.Patches).To(ContainElement(
				jsonpatch.Operation{Operation: "add", Path: "/spec/updateStrategy/type", Value: "RollingUpdate"},
			))

			// Does not work because no deepequal check (value is a map/reference)
			//Expect(response.Patches).To(ContainElement(
			//	jsonpatch.Operation{Operation: "add", Path: "/spec/updateStrategy/rollingUpdate", Value: map[string]interface{}{"partition": 0}},
			//))

			// Does not work because of unix timestamp
			//Expect(response.Patches).To(ContainElement(
			//	jsonpatch.Operation{Operation: "add", Path: "/metadata/annotations/quarks.cloudfoundry.org~1update-start-time", Value: "1574265011"},
			//))

			Expect(response.AdmissionResponse.Allowed).To(BeTrue())
		})
	})

	Context("with an invalid admissions request content", func() {
		BeforeEach(func() {
			raw, _ := json.Marshal(old)

			request = admission.Request{
				AdmissionRequest: admissionv1beta1.AdmissionRequest{
					OldObject: runtime.RawExtension{Raw: raw},
					Object:    runtime.RawExtension{Raw: []byte("invalid")},
				},
			}
		})

		It("bad request should be the response", func() {
			response := mutator.Handle(ctx, request)
			Expect(response.AdmissionResponse.Allowed).To(BeFalse())
			Expect(response.AdmissionResponse.Result.Code).To(Equal(int32(400)))
			Expect(response.Patches).To(BeEmpty())
		})
	})
})
