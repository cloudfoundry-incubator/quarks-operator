package extendedstatefulset

import (
	"context"
	"net/http"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllersconfig"
)

// PodMutator changes pod definitions
type PodMutator struct {
	client       client.Client
	scheme       *runtime.Scheme
	setReference setReferenceFunc
	log          *zap.SugaredLogger
	ctrConfig    *controllersconfig.ControllersConfig
	decoder      types.Decoder
}

// Implement admission.Handler so the controller can handle admission request.
var _ admission.Handler = &PodMutator{}

// NewPodMutator returns a new reconcile.Reconciler
func NewPodMutator(log *zap.SugaredLogger, ctrConfig *controllersconfig.ControllersConfig, mgr manager.Manager, srf setReferenceFunc) admission.Handler {
	reconcilerLog := log.Named("extendedstatefulset-pod1-mutator")
	reconcilerLog.Info("Creating a Pod mutator for ExtendedStatefulSet")

	return &PodMutator{
		log:          reconcilerLog,
		ctrConfig:    ctrConfig,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		setReference: srf,
	}
}

// Handle manages volume claims for ExtendedStatefulSet pods
func (m *PodMutator) Handle(ctx context.Context, req types.Request) types.Response {
	pod := &corev1.Pod{}

	err := m.decoder.Decode(req, pod)

	m.log.Debug("Pod mutator handler ran for pod ", pod.Name)

	if err != nil {
		return admission.ErrorResponse(http.StatusBadRequest, err)
	}

	updatedPod := pod.DeepCopy()

	// TODO: test to see if this is a pod we want to mutate
	// if strings.HasPrefix(pod.Name, "") {
	// 	err = m.mutatePodsFn(ctx, updatedPod)
	// 	if err != nil {
	// 		return admission.ErrorResponse(http.StatusInternalServerError, err)
	// 	}
	// }

	return admission.PatchResponse(pod, updatedPod)
}

// mutatePodsFn add an annotation to the given pod
func (m *PodMutator) mutatePodsFn(ctx context.Context, pod *corev1.Pod) error {
	m.log.Info("Mutating Pod ", pod.Name)

	// TODO: add/remove volumes and volume mounts from pods
	// pod.Spec.Volumes = append(
	// 	pod.Spec.Volumes,
	// 	corev1.Volume{},
	// )

	// pod.Spec.Containers[0].VolumeMounts = append(
	// 	pod.Spec.Containers[0].VolumeMounts,
	// 	corev1.VolumeMount{},
	// )

	return nil
}

// podAnnotator implements inject.Client.
// A client will be automatically injected.
var _ inject.Client = &PodMutator{}

// InjectClient injects the client.
func (m *PodMutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// podAnnotator implements inject.Decoder.
// A decoder will be automatically injected.
var _ inject.Decoder = &PodMutator{}

// InjectDecoder injects the decoder.
func (m *PodMutator) InjectDecoder(d types.Decoder) error {
	m.decoder = d
	return nil
}
