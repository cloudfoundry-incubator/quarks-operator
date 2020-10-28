package waitservice

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/operatorimage"
	"github.com/pkg/errors"
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
	var services []string

	if err := json.Unmarshal([]byte(annotations[WaitKey]), &services); err == nil && len(services) != 0 {
		valid = true
	}

	return valid
}

// Handle checks if the pod has the "wait-for" annotation and injects an initcontainer waiting for the service
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	if err := m.decoder.Decode(req, pod); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	updatedPod := pod.DeepCopy()
	if validWait(pod.GetAnnotations()) {
		if err := m.addInitContainer(updatedPod); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	marshaledPod, err := json.Marshal(updatedPod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (m *PodMutator) addInitContainer(pod *corev1.Pod) error {
	annotations := pod.GetAnnotations()
	servicesStr, ok := annotations[WaitKey]
	if !ok {
		return fmt.Errorf("no annotations %s found", WaitKey)
	}

	var services []*string
	if err := json.Unmarshal([]byte(servicesStr), &services); err != nil {
		return errors.Wrapf(err, "failed unmarshalling services in '%s'", WaitKey)
	}

	pod.Spec.InitContainers = append(createWaitContainers(services...), pod.Spec.InitContainers...)

	return nil
}

func createWaitContainers(requiredServices ...*string) []corev1.Container {
	containers := []corev1.Container{}
	for _, service := range requiredServices {
		if service == nil {
			continue
		}
		containers = append(containers, corev1.Container{Name: fmt.Sprintf("wait-for-%s", *service),
			Image:   operatorimage.GetOperatorDockerImage(),
			Command: []string{"/usr/bin/dumb-init", "--"},
			Args: []string{
				"/bin/sh",
				"-xc",
				fmt.Sprintf("time quarks-operator util wait %s", *service),
			}})
	}
	return containers
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
