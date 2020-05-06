package waitservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/operatorimage"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"code.cloudfoundry.org/quarks-utils/pkg/config"
)

var (
	// WaitKey is the key for identifying which service the pod has to wait for
	WaitKey = fmt.Sprintf("%s/wait-for", apis.GroupName)
)

// PodMutator for adding waiting init containers for linked services
type PodMutator struct {
	client  client.Client
	log     *zap.SugaredLogger
	config  *config.Config
	decoder *admission.Decoder
}

// Check that PodMutator implements the admission.Handler interface
var _ admission.Handler = &PodMutator{}

// NewPodMutator returns a new mutator that adds waiting InitContainers
func NewPodMutator(log *zap.SugaredLogger, config *config.Config) admission.Handler {
	mutatorLog := log.Named("waitservice-pod-mutator")
	mutatorLog.Info("Creating a Pod mutator for WaitService")

	return &PodMutator{
		log:    mutatorLog,
		config: config,
	}
}

func validWait(annotations map[string]string) bool {
	valid := false
	if annotations[WaitKey] != "" {
		valid = true
	}

	return valid
}

// Handle checks if the pod has the "wait-for" annotation and injects an initcontainer waiting for the service
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	updatedPod := pod.DeepCopy()
	if validWait(pod.GetAnnotations()) {
		err = m.addInitContainer(ctx, updatedPod)
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

func (m *PodMutator) addInitContainer(ctx context.Context, pod *corev1.Pod) error {
	annotations := pod.GetAnnotations()
	serviceName, ok := annotations[WaitKey]
	if !ok {
		return fmt.Errorf("no annotations %s found", WaitKey)
	}

	if serviceName == "" {
		return fmt.Errorf("service name in '%s' empty", WaitKey)
	}

	pod.Spec.InitContainers = append(pod.Spec.InitContainers, createWaitContainer(&serviceName)...)

	return nil
}

func createWaitContainer(requiredService *string) []corev1.Container {
	if requiredService == nil {
		return nil
	}
	return []corev1.Container{{
		Name:    "wait-for",
		Image:   operatorimage.GetOperatorDockerImage(),
		Command: []string{"/usr/bin/dumb-init", "--"},
		Args: []string{
			"/bin/sh",
			"-xc",
			fmt.Sprintf("time cf-operator util wait %s", *requiredService),
		},
	}}

}

// Check that PodMutator implements the inject.Client interface
var _ inject.Client = &PodMutator{}

// InjectClient injects the client.
func (m *PodMutator) InjectClient(c client.Client) error {
	m.client = c
	return nil
}

// Check that PodMutator implements the admission.DecoderInjector interface
var _ admission.DecoderInjector = &PodMutator{}

// InjectDecoder injects the decoder.
func (m *PodMutator) InjectDecoder(d *admission.Decoder) error {
	m.decoder = d
	return nil
}
