package quarksstatefulset

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	mTypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// statefulSetKind is the kind name of statefulSet
const statefulSetKind = "StatefulSet"

// PodMutator changes pod definitions
type PodMutator struct {
	client  client.Client
	log     *zap.SugaredLogger
	config  *config.Config
	decoder *admission.Decoder
}

// Check that PodMutator implements the admission.Handler interface
var _ admission.Handler = &PodMutator{}

// NewPodMutator returns a new reconcile.Reconciler
func NewPodMutator(log *zap.SugaredLogger, config *config.Config) admission.Handler {
	mutatorLog := log.Named("quarks-statefulset-pod-mutator")
	mutatorLog.Info("Creating a Pod mutator for QuarksStatefulSet")

	return &PodMutator{
		log:    mutatorLog,
		config: config,
	}
}

// Handle manages volume claims for QuarksStatefulSet pods
func (m *PodMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	err := m.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	m.log.Debug("Pod mutator handler ran for pod ", pod.Name)

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

// mutatePodsFn add an annotation to the given pod
func (m *PodMutator) mutatePodsFn(ctx context.Context, pod *corev1.Pod) error {

	m.log.Info("Mutating Pod ", pod.Name)

	// Check if it is a volumeManagement statefulSet pod
	if !isVolumeManagementStatefulSetPod(pod.Name) {
		// Fetch statefulSet
		statefulSet, err := m.fetchStatefulSet(ctx, pod.GetOwnerReferences())
		if err != nil {
			return errors.Wrapf(err, "Couldn't fetch StatefulSet of pod %s", pod.Name)
		}

		// Fetch quarksStatefulSet
		quarksStatefulSet, err := m.fetchQuarksStatefulSet(ctx, pod.Labels)
		if err != nil {
			return errors.Wrapf(err, "Couldn't fetch QuarksStatefulSet of pod %s", pod.Name)
		}

		// Check if it has volumeClaimTemplates
		if quarksStatefulSet.Spec.Template.Spec.VolumeClaimTemplates != nil {
			err := m.addPersistentVolumeClaims(ctx, statefulSet, quarksStatefulSet, pod)
			if err != nil {
				return errors.Wrapf(err, "Adding volume spec has failed for pod %s", pod.Name)
			}
		}
	}

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

// addPersistentVolumeClaims adds volume spec to pods for persistent volume claims
func (m *PodMutator) addPersistentVolumeClaims(ctx context.Context, statefulSet *v1beta2.StatefulSet, quarksStatefulSet *qstsv1a1.QuarksStatefulSet, pod *corev1.Pod) error {

	// Get persistentVolumeClaims list
	opts := client.InNamespace(m.config.Namespace)
	persistentVolumeClaimList := &corev1.PersistentVolumeClaimList{}
	err := m.client.List(ctx, persistentVolumeClaimList, opts)
	if err != nil {
		return errors.Wrapf(err, "Couldn't fetch PVC's.")
	}

	// Get VolumeClaimTemplates list
	volumeClaimTemplates := quarksStatefulSet.Spec.Template.Spec.VolumeClaimTemplates

	volumeClaimTemplatesMap := make(map[string]corev1.PersistentVolumeClaim, len(volumeClaimTemplates))
	for _, volumeClaimTemplate := range volumeClaimTemplates {
		volumeClaimTemplatesMap[volumeClaimTemplate.Name] = volumeClaimTemplate
	}

	volumeMap := make(map[string]corev1.Volume, len(pod.Spec.Volumes))
	for _, volume := range pod.Spec.Volumes {
		volumeMap[volume.Name] = volume
	}

	m.addVolumeSpec(pod, volumeClaimTemplatesMap, volumeMap, quarksStatefulSet.Name)

	return nil
}

// addVolumeSpec adds volume spec to the pod container volumes spec
func (m *PodMutator) addVolumeSpec(pod *corev1.Pod, volumeClaimTemplatesMap map[string]corev1.PersistentVolumeClaim, volumeMap map[string]corev1.Volume, quarksStatefulSetName string) {

	for _, container := range pod.Spec.Containers {
		for _, volumeMount := range container.VolumeMounts {

			_, foundVolumeClaimTemplate := volumeClaimTemplatesMap[volumeMount.Name]
			if foundVolumeClaimTemplate {
				podOrdinal := names.OrdinalFromPodName(pod.GetName())
				persistentVolumeClaim := fmt.Sprintf("%s-%s-%s-%d", volumeMount.Name, "volume-management", quarksStatefulSetName, podOrdinal)

				volume, foundVolume := volumeMap[volumeMount.Name]
				if !foundVolume {
					persistentVolumeClaimVolumeSource := corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: persistentVolumeClaim,
					}
					volume := corev1.Volume{
						Name: volumeMount.Name,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &persistentVolumeClaimVolumeSource,
						},
					}
					pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
					volumeMap[volume.Name] = volume
					m.log.Info("Added volume spec with persistent volume claim ", persistentVolumeClaim, " to Pod ", pod.Name)
				} else {
					for podVolumeIndex, podVolume := range pod.Spec.Volumes {
						if podVolume.Name == volume.Name {
							pod.Spec.Volumes[podVolumeIndex].PersistentVolumeClaim.ClaimName = persistentVolumeClaim
						}
					}
				}
			}
		}
	}
}

// isVolumeManagementStatefulSetPod checks if it is pod of the volumeManagement statefulSet
func isVolumeManagementStatefulSetPod(podName string) bool {
	return strings.HasPrefix(podName, "volume-management")
}

// fetchQuarksStatefulSet fetches the quarksStatefulSet of the pod
func (m *PodMutator) fetchStatefulSet(ctx context.Context, ownerReferences []metav1.OwnerReference) (*v1beta2.StatefulSet, error) {
	// Find statefulSet name from ownerReferences
	statefulSetName := ""
	for _, ref := range ownerReferences {
		if ref.Kind == statefulSetKind {
			statefulSetName = ref.Name
		}
	}

	statefulSet := &v1beta2.StatefulSet{}
	key := mTypes.NamespacedName{Namespace: m.config.Namespace, Name: statefulSetName}
	err := m.client.Get(ctx, key, statefulSet)
	if err != nil {
		return &v1beta2.StatefulSet{}, err
	}
	return statefulSet, nil
}

// fetchQuarksStatefulSet fetches the quarksStatefulSet of the pod
func (m *PodMutator) fetchQuarksStatefulSet(ctx context.Context, podLabels map[string]string) (*qstsv1a1.QuarksStatefulSet, error) {
	// Find quarksStatefulSet name from labels
	quarksStatefulSetName, ok := podLabels[qstsv1a1.LabelQStsName]
	if !ok {
		return &qstsv1a1.QuarksStatefulSet{}, errors.New("Couldn't fetch name of quarksStatefulSet from pod labels")
	}

	quarksStatefulSet := &qstsv1a1.QuarksStatefulSet{}
	key := mTypes.NamespacedName{Namespace: m.config.Namespace, Name: quarksStatefulSetName}
	err := m.client.Get(ctx, key, quarksStatefulSet)
	if err != nil {
		return &qstsv1a1.QuarksStatefulSet{}, err
	}
	return quarksStatefulSet, nil
}

// isQuarksStatefulSet check is it is quarksStatefulSet Pod
func isQuarksStatefulSet(labels map[string]string) bool {
	if _, exists := labels[appsv1.StatefulSetPodNameLabel]; exists {
		return true
	}
	return false
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
