package quarksstatefulset

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// PodMutator for adding the pod-ordinal label on statefulset pods
type PodMutator struct {
	log     *zap.SugaredLogger
	config  *config.Config
	decoder *admission.Decoder
}

// Check that PodMutator implements the admission.Handler interface
var _ admission.Handler = &PodMutator{}

// NewPodMutator returns a pod mutator to add pod-ordinal on statefulset pods
func NewPodMutator(log *zap.SugaredLogger, config *config.Config) admission.Handler {
	mutatorLog := log.Named("quarks-statefulset-pod-mutator")
	mutatorLog.Info("Creating a Pod mutator for QuarksStatefulSet")

	return &PodMutator{
		log:    mutatorLog,
		config: config,
	}
}

// Handle checks if pod is part of a statefulset and adds the pod-ordinal label
// on the pod
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	m.log.Debugf("Pod mutator handler ran for pod '%s/%s'", pod.Namespace, pod.Name)

	updatedPod := pod.DeepCopy()
	if isQuarksStatefulSet(pod.GetLabels()) {
		err = m.mutatePodsFn(ctx, updatedPod)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	marshaledPod, err := json.Marshal(updatedPod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// mutatePodsFn add a pod-ordinal label to the given pod
func (m *PodMutator) mutatePodsFn(ctx context.Context, pod *corev1.Pod) error {

	m.log.Infof("Mutating Pod '%s/%s'", pod.Namespace, pod.Name)

	// Add pod ordinal label for service selectors
	podLabels := pod.GetLabels()
	if podLabels == nil {
		podLabels = map[string]string{}
	}

	podOrdinal := names.OrdinalFromPodName(pod.GetName())
	if podOrdinal != -1 {
		podLabels[qstsv1a1.LabelPodOrdinal] = strconv.Itoa(podOrdinal)
		pod.SetLabels(podLabels)
	}

	return nil
}

// isQuarksStatefulSet check is it is quarksStatefulSet Pod
func isQuarksStatefulSet(labels map[string]string) bool {
	if _, exists := labels[appsv1.StatefulSetPodNameLabel]; exists {
		return true
	}
	return false
}

// Check that PodMutator implements the admission.DecoderInjector interface
var _ admission.DecoderInjector = &PodMutator{}

// InjectDecoder injects the decoder.
func (m *PodMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
