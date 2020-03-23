package quarkslink

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
)

// PodMutator for mounting quark link secrets on entangled pods
type PodMutator struct {
	client  client.Client
	log     *zap.SugaredLogger
	config  *config.Config
	decoder *admission.Decoder
}

// Check that PodMutator implements the admission.Handler interface
var _ admission.Handler = &PodMutator{}

// NewPodMutator returns a new mutator to mount secrets on entangled pods
func NewPodMutator(log *zap.SugaredLogger, config *config.Config) admission.Handler {
	mutatorLog := log.Named("quarks-link-pod-mutator")
	mutatorLog.Info("Creating a Pod mutator for QuarksLink")

	return &PodMutator{
		log:    mutatorLog,
		config: config,
	}
}

// Handle checks if the pod is an entangled pod and mounts the quarkslink secret, returns
// the unmodified pod otherwise
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	updatedPod := pod.DeepCopy()
	if validEntanglement(pod.GetAnnotations()) {
		m.log.Debugf("Adding quarks link secret to entangled pod '%s'", pod.Name)
		err = m.addSecrets(ctx, req.Namespace, updatedPod)
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

func (m *PodMutator) addSecrets(ctx context.Context, namespace string, pod *corev1.Pod) error {
	e := newEntanglement(pod.GetAnnotations())
	links, err := m.findLinks(ctx, namespace, e)
	if err != nil {
		m.log.Errorf("Couldn't list entanglement secrets for '%s/%s' in %s", e.deployment, e.consumes, namespace)
		return err
	}

	if len(links) == 0 {
		return fmt.Errorf("couldn't find any entanglement secret for deployment '%s' in %s", e.deployment, namespace)
	}

	// add missing volume sources to pod
	for _, link := range links {
		if !hasSecretVolumeSource(pod.Spec.Volumes, link.secret.Name) {
			volume := corev1.Volume{
				Name: link.secret.Name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: link.secret.Name,
					},
				},
			}
			pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
		}

		// create/update volume mount on containers
		mount := corev1.VolumeMount{
			Name:      link.secret.Name,
			ReadOnly:  true,
			MountPath: filepath.Join("/quarks/link", e.deployment, link.String()),
		}
		for i, container := range pod.Spec.Containers {
			idx := findVolumeMount(container.VolumeMounts, link.secret.Name)
			if idx > -1 {
				container.VolumeMounts[idx] = mount
			} else {
				container.VolumeMounts = append(container.VolumeMounts, mount)
			}
			pod.Spec.Containers[i] = container
		}

		// add link properties as environment variables
		keys := []string{}
		for key := range link.secret.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for contIdx := range pod.Spec.Containers {
			for _, key := range keys {
				pod.Spec.Containers[contIdx].Env = append(pod.Spec.Containers[contIdx].Env,
					corev1.EnvVar{
						Name: fmt.Sprintf("LINK_%s", asEnvironmentVariableName(key)),
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: link.secret.Name},
								Key:                  key,
							},
						},
					},
				)
			}
		}
	}

	return nil
}

func (m *PodMutator) findLinks(ctx context.Context, namespace string, e entanglement) (links, error) {
	links := []link{}

	list := &corev1.SecretList{}
	// can't use entanglement labels, because quarks-job does not set
	// labels per container, so we list all secrets from the deployment
	labels := map[string]string{bdv1.LabelDeploymentName: e.deployment}
	err := m.client.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels(labels))
	if err != nil {
		return links, err
	}

	if len(list.Items) == 0 {
		return links, nil
	}

	for i := range list.Items {
		if link, ok := e.find(list.Items[i]); ok {
			link.secret = &(list.Items[i])
			links = append(links, link)
		}
	}

	return links, nil
}

func asEnvironmentVariableName(input string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return strings.ToUpper(reg.ReplaceAllString(input, "_"))
}

func hasSecretVolumeSource(volumes []corev1.Volume, name string) bool {
	for _, v := range volumes {
		if v.Secret != nil && v.Secret.SecretName == name {
			return true
		}
	}
	return false
}

func findVolumeMount(mounts []corev1.VolumeMount, name string) int {
	for i, m := range mounts {
		if m.Name == name {
			return i
		}
	}
	return -1
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
