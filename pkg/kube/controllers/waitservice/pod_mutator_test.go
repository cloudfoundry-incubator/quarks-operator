package waitservice_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
	"gomodules.xyz/jsonpatch/v2"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/waitservice"
	"code.cloudfoundry.org/quarks-operator/testing"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	helper "code.cloudfoundry.org/quarks-utils/testing/testhelper"
)

var _ = Describe("Adds waiting initcontainer on pods with wait-for annotation", func() {
	var (
		client             client.Client
		ctx                context.Context
		decoder            *admission.Decoder
		entanglementSecret corev1.Secret
		env                testing.Catalog
		log                *zap.SugaredLogger
		mutator            admission.Handler
		pod                corev1.Pod
		request            admission.Request
		response           admission.Response
	)

	podPatch := `{"op":"add","path":"/spec/initContainers","value":[{"args":["/bin/sh","-xc","time quarks-operator util wait test"],"command":["/usr/bin/dumb-init","--"],"name":"wait-for-test","resources":{}}]}`
	podPatch2 := `{"op":"add","path":"/spec/initContainers","value":[{"args":["/bin/sh","-xc","time quarks-operator util wait test"],"command":["/usr/bin/dumb-init","--"],"name":"wait-for-test","resources":{}},{"args":["/bin/sh","-xc","time quarks-operator util wait test2"],"command":["/usr/bin/dumb-init","--"],"name":"wait-for-test2","resources":{}}]}`

	jsonPatches := func(operations []jsonpatch.Operation) []string {
		patches := make([]string, len(operations))
		for i, patch := range operations {
			patches[i] = patch.Json()
		}
		return patches
	}

	newAdmissionRequest := func(pod corev1.Pod) admission.Request {
		raw, _ := json.Marshal(pod)
		return admission.Request{
			AdmissionRequest: admissionv1beta1.AdmissionRequest{
				Object: runtime.RawExtension{Raw: raw},
			},
		}
	}

	BeforeEach(func() {
		_, log = helper.NewTestLogger()
		ctx = ctxlog.NewParentContext(log)

		mutator = waitservice.NewPodMutator(log, &config.Config{CtxTimeOut: 10 * time.Second})

		scheme := runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		decoder, _ = admission.NewDecoder(scheme)
		mutator.(admission.DecoderInjector).InjectDecoder(decoder)

	})

	JustBeforeEach(func() {
		mutator.(inject.Client).InjectClient(client)
		response = mutator.Handle(ctx, request)
	})

	Context("when pod has no special annotation", func() {
		BeforeEach(func() {
			pod = env.DefaultPod("test-pod")
			request = newAdmissionRequest(pod)
			client = fakeClient.NewFakeClient(&entanglementSecret)
		})

		It("does not apply changes", func() {
			Expect(response.AdmissionResponse.Allowed).To(BeTrue())
			Expect(response.Patches).To(BeEmpty())
		})
	})

	Context("when valid label exists on pod", func() {
		BeforeEach(func() {
			pod = env.AnnotatedPod("waiting-pod", map[string]string{
				waitservice.WaitKey: "[\"test\"]",
			})
			pod.Spec.Containers = []corev1.Container{
				{Name: "first", Image: "busybox", Command: []string{"sleep", "3600"}},
				{Name: "second", Image: "busybox", Command: []string{"sleep", "3600"}},
			}
			request = newAdmissionRequest(pod)
		})

		Context("when wait-for label exists", func() {
			BeforeEach(func() {
				client = fakeClient.NewFakeClient(&entanglementSecret)
			})

			It("initcontainer is appended", func() {
				Expect(response.Allowed).To(BeTrue(), response.Result.String())

				Expect(response.Patches).To(HaveLen(1))
				patches := jsonPatches(response.Patches)
				Expect(patches).To(ContainElement(podPatch))

				Expect(response.AdmissionResponse.Allowed).To(BeTrue())
			})
		})
	})

	Context("when more than one service is specified", func() {
		BeforeEach(func() {
			pod = env.AnnotatedPod("waiting-pod", map[string]string{
				waitservice.WaitKey: "[\"test\", \"test2\"]",
			})
			pod.Spec.Containers = []corev1.Container{
				{Name: "first", Image: "busybox", Command: []string{"sleep", "3600"}},
				{Name: "second", Image: "busybox", Command: []string{"sleep", "3600"}},
			}
			request = newAdmissionRequest(pod)
		})

		Context("when wait-for label exists", func() {
			BeforeEach(func() {
				client = fakeClient.NewFakeClient(&entanglementSecret)
			})

			It("initcontainer is appended", func() {
				Expect(response.Allowed).To(BeTrue(), response.Result.String())

				Expect(response.Patches).To(HaveLen(1))
				patches := jsonPatches(response.Patches)
				Expect(patches).To(ContainElement(podPatch2))

				Expect(response.AdmissionResponse.Allowed).To(BeTrue())
			})
		})
	})

	Context("when invalid label exists on pod", func() {
		BeforeEach(func() {
			pod = env.AnnotatedPod("waiting-pod", map[string]string{
				waitservice.WaitKey: "test",
			})
			request = newAdmissionRequest(pod)
		})

		Context("when wait-for label exists", func() {
			BeforeEach(func() {
				client = fakeClient.NewFakeClient(&entanglementSecret)
			})

			It("initcontainer is not appended", func() {
				Expect(response.Allowed).To(BeTrue(), response.Result.String())

				Expect(response.Patches).To(HaveLen(0))
				Expect(response.AdmissionResponse.Allowed).To(BeTrue())
			})
		})
	})

	Context("when pod has existing initcontainers", func() {
		podPatch := []string{
			"{\"op\":\"add\",\"path\":\"/spec/initContainers/2\",\"value\":{\"command\":[\"sleep\",\"3600\"],\"image\":\"busybox\",\"name\":\"second\",\"resources\":{}}}",
			"{\"op\":\"replace\",\"path\":\"/spec/initContainers/0/name\",\"value\":\"wait-for-test\"}",
			"{\"op\":\"replace\",\"path\":\"/spec/initContainers/0/command/1\",\"value\":\"--\"}",
			"{\"op\":\"replace\",\"path\":\"/spec/initContainers/0/command/0\",\"value\":\"/usr/bin/dumb-init\"}",
			"{\"op\":\"add\",\"path\":\"/spec/initContainers/0/args\",\"value\":[\"/bin/sh\",\"-xc\",\"time quarks-operator util wait test\"]}",
			"{\"op\":\"remove\",\"path\":\"/spec/initContainers/0/image\"}",
			"{\"op\":\"replace\",\"path\":\"/spec/initContainers/1/name\",\"value\":\"first\"}",
		}

		BeforeEach(func() {
			pod = env.AnnotatedPod("waiting-pod", map[string]string{
				waitservice.WaitKey: "[\"test\"]",
			})
			pod.Spec.InitContainers = []corev1.Container{
				{Name: "first", Image: "busybox", Command: []string{"sleep", "3600"}},
				{Name: "second", Image: "busybox", Command: []string{"sleep", "3600"}},
			}
			request = newAdmissionRequest(pod)

			client = fakeClient.NewFakeClient(&entanglementSecret)
		})

		It("does add the initcontainer at the beginning, and not replace it", func() {
			Expect(response.Allowed).To(BeTrue(), response.Result.String())
			Expect(response.Patches).To(HaveLen(7))
			patches := jsonPatches(response.Patches)
			for _, p := range podPatch {
				Expect(patches).To(ContainElement(p))
			}
			Expect(response.AdmissionResponse.Allowed).To(BeTrue())
		})

	})
})
