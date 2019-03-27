package extendedstatefulset

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	mTypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

// PodMutator changes pod definitions
type PodMutator struct {
	client       client.Client
	scheme       *runtime.Scheme
	setReference setReferenceFunc
	log          *zap.SugaredLogger
	config       *config.Config
	decoder      types.Decoder
}

// Implement admission.Handler so the controller can handle admission request.
var _ admission.Handler = &PodMutator{}

// NewPodMutator returns a new reconcile.Reconciler
func NewPodMutator(log *zap.SugaredLogger, config *config.Config, mgr manager.Manager, srf setReferenceFunc) admission.Handler {
	mutatorLog := log.Named("extendedstatefulset-pod1-mutator")
	mutatorLog.Info("Creating a Pod mutator for ExtendedStatefulSet")

	return &PodMutator{
		log:          mutatorLog,
		config:       config,
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

	// TODO :- send pod instead of annotations.

	if isStatefulSetPod(pod.GetLabels()) {
		err = m.mutatePodsFn(ctx, updatedPod)
		if err != nil {
			return admission.ErrorResponse(http.StatusInternalServerError, err)
		}
	}

	return admission.PatchResponse(pod, updatedPod)
}

// mutatePodsFn add an annotation to the given pod
func (m *PodMutator) mutatePodsFn(ctx context.Context, pod *corev1.Pod) error {
	m.log.Info("Mutating Pod ", pod.Name)

	// Fetch statefulSet
	statefulSetName := getStatefulSetName(pod.Name)
	statefulSet := &v1beta2.StatefulSet{}
	key := mTypes.NamespacedName{Namespace: m.config.Namespace, Name: statefulSetName}
	err := m.client.Get(ctx, key, statefulSet)
	if err != nil {
		return errors.Wrapf(err, "Couldn't fetch Statefulset")
	}

	volumeClaimTemplateList := statefulSet.Spec.VolumeClaimTemplates

	// check if VolumeClaimTemplate is present
	if volumeClaimTemplateList != nil {

		// Get persistentVolumeClaims list
		opts := client.InNamespace(m.config.Namespace)
		pvcList := &corev1.PersistentVolumeClaimList{}
		err := m.client.List(ctx, opts, pvcList)
		if err != nil {
			return errors.Wrapf(err, "Couldn't fetch PVC's")
		}

		// Loop over volumeClaimTemplates
		for _, volumeClaimTemplate := range volumeClaimTemplateList {

			currentVersionInt := getVersionFromName(pod.Name, 2)
			minVersion := math.MaxInt64
			minPVCName := ""
			// loop over pvclist to find the earliest one
			for _, pvc := range pvcList.Items {
				pvcName := strings.Split(pvc.GetName(), getNameWithOutVersion(pod.Name, 2))[0]
				pvcName = pvcName[:len(pvcName)-1]
				if getNameWithOutVersion(pvcName, 1) == getNameWithOutVersion(volumeClaimTemplate.Name, 1) && pvc.Name[len(pvc.Name)-1:] == pod.Name[len(pod.Name)-1:] {
					pvcVersion := getVersionFromName(pvc.Name, 2)
					if minVersion > pvcVersion {
						minVersion = pvcVersion
						minPVCName = pvc.Name
					}
				}
			}
			if minVersion != currentVersionInt {
				changeVolumePods(minPVCName, pod, minVersion, currentVersionInt, &volumeClaimTemplate)
			}
		}
	}
	return nil
}

// changeVolumePods will append volume and change volumeMount name
func changeVolumePods(desiredPVCName string, pod *corev1.Pod, desiredVersionInt int, currentVersionInt int, volumeClaimTemplate *corev1.PersistentVolumeClaim) {

	// generate desired vct name
	desiredVersion := fmt.Sprintf("%s%d", "v", desiredVersionInt)
	desiredVCTName := replaceVersionInName(volumeClaimTemplate.Name, desiredVersion, 1)

	appendVolumetoPod(pod, volumeClaimTemplate, desiredVCTName, desiredPVCName)
	removeUnusedVolumes(pod, desiredVCTName, volumeClaimTemplate.Name)
}

func removeUnusedVolumes(pod *corev1.Pod, desiredVCTName string, currentVCTName string) {
	for indexV, volume := range pod.Spec.Volumes {
		if getNameWithOutVersion(volume.Name, 1) == getNameWithOutVersion(currentVCTName, 1) && volume.Name != desiredVCTName && volume.Name != currentVCTName {
			// delete this spec
			pod.Spec.Volumes[indexV] = pod.Spec.Volumes[len(pod.Spec.Volumes)-1]
			pod.Spec.Volumes = pod.Spec.Volumes[:len(pod.Spec.Volumes)-1]
		}
	}
}

// getNameWithOutVersion returns name removing the version index
func getNameWithOutVersion(name string, offset int) string {
	nameSplit := strings.Split(name, "-")
	nameSplit = nameSplit[0 : len(nameSplit)-offset]
	name = strings.Join(nameSplit, "-")
	return name
}

// isStatefulSetPod check is it is extendedstatefulset Pod
func isStatefulSetPod(labels map[string]string) bool {
	if _, exists := labels["statefulset.kubernetes.io/pod-name"]; exists {
		return true
	}
	return false
}

// getStatefulSetName gets statefulsetname from podName
func getStatefulSetName(name string) string {
	nameSplit := strings.Split(name, "-")
	nameSplit = nameSplit[0 : len(nameSplit)-1]
	statefulSetName := strings.Join(nameSplit, "-")
	return statefulSetName
}

// getVersionFromName fetches version from name
func getVersionFromName(name string, offset int) int {
	nameSplit := strings.Split(name, "-")
	version := string(nameSplit[len(nameSplit)-offset][1])
	versionInt, err := strconv.Atoi(version)
	if err != nil {
		errors.Wrapf(err, "Atoi failed to convert")
	}
	return versionInt
}

// replaceVersionInName replaces with the given version in name at offset
func replaceVersionInName(name string, version string, offset int) string {
	nameSplit := strings.Split(name, "-")
	nameSplit[len(nameSplit)-offset] = version
	name = strings.Join(nameSplit, "-")
	return name
}

// appendVolumetoPod appends desiredvolume to pod
func appendVolumetoPod(pod *corev1.Pod, volumeClaimTemplate *corev1.PersistentVolumeClaim, desiredVCTName string, desiredPVCName string) {
	// Find the desired volume and append new volume
	podVolumes := pod.Spec.Volumes
	for _, podVolume := range podVolumes {
		if podVolume.Name == volumeClaimTemplate.Name {
			desiredVolume := podVolume.DeepCopy()
			desiredVolume.Name = desiredVCTName
			desiredVolume.PersistentVolumeClaim.ClaimName = desiredPVCName
			pod.Spec.Volumes = append(pod.Spec.Volumes, *desiredVolume)

			// Change volume mount names
			changeVolumeMountNames(pod, podVolume.Name, desiredVCTName)

			// TODO delete unused PVC volumes
		}
	}
}

// changeVolumeMountNames replaces name of volumeMount with desired volume's name
func changeVolumeMountNames(pod *corev1.Pod, volumeName string, desiredName string) {
	for indexC, container := range pod.Spec.Containers {
		for indexV, volumeMount := range container.VolumeMounts {
			if volumeMount.Name == volumeName {
				pod.Spec.Containers[indexC].VolumeMounts[indexV].Name = desiredName
			}
		}
	}
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
