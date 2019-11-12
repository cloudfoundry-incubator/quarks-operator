package extendedstatefulset

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	essv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	podutil "code.cloudfoundry.org/quarks-utils/pkg/pod"
)

// alterVolumeManagementStatefulSet creates the volumeManagement statefulSet for persistent volume claim creation.
func (r *ReconcileExtendedStatefulSet) alterVolumeManagementStatefulSet(ctx context.Context, currentVersion int, desiredVersion int, exStatefulSet *essv1a1.ExtendedStatefulSet, currentStatefulSet *v1beta2.StatefulSet) error {
	// Create volumeManagement statefulSet if it is the first time extendedStatefulSet is created.
	if currentVersion == 0 && currentVersion != desiredVersion {
		err := r.createVolumeManagementStatefulSets(ctx, exStatefulSet, currentStatefulSet)
		if err != nil {
			return errors.Wrapf(err, "Creation of volumeManagement statefulSets failed.")
		}
	} else {
		replicaDifference := r.getReplicaDifference(exStatefulSet, currentStatefulSet)
		if replicaDifference > 0 {
			err := r.createVolumeManagementStatefulSets(ctx, exStatefulSet, currentStatefulSet)
			if err != nil {
				return errors.Wrapf(err, "Creation of VolumeManagement StatefulSets failed")
			}
		}
	}
	return nil
}

// getReplicaDifference calculates the difference between replica count
func (r *ReconcileExtendedStatefulSet) getReplicaDifference(exStatefulSet *essv1a1.ExtendedStatefulSet, statefulSet *v1beta2.StatefulSet) int {
	return int(*exStatefulSet.Spec.Template.Spec.Replicas) - int(*statefulSet.Spec.Replicas)
}

// createVolumeManagementStatefulSet creates a volumeManagement statefulSet
func (r *ReconcileExtendedStatefulSet) createVolumeManagementStatefulSets(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet, statefulSet *v1beta2.StatefulSet) error {

	var desiredVolumeManagementStatefulSets []v1beta2.StatefulSet

	template := exStatefulSet.Spec.Template
	template.SetName("volume-management")

	// Place the StatefulSet in the same namespace as the ExtendedStatefulSet
	template.SetNamespace(exStatefulSet.Namespace)

	if exStatefulSet.Spec.ZoneNodeLabel == "" {
		exStatefulSet.Spec.ZoneNodeLabel = essv1a1.DefaultZoneNodeLabel
	}

	if len(exStatefulSet.Spec.Zones) > 0 {
		for zoneIndex, zoneName := range exStatefulSet.Spec.Zones {
			statefulSet, err := r.generateVolumeManagementSingleStatefulSet(exStatefulSet, &template, zoneIndex, zoneName)
			if err != nil {
				return errors.Wrapf(err, "Could not generate volumeManagement StatefulSet template for AZ '%d/%s'", zoneIndex, zoneName)
			}
			desiredVolumeManagementStatefulSets = append(desiredVolumeManagementStatefulSets, *statefulSet)
		}
	} else {
		statefulSet, err := r.generateVolumeManagementSingleStatefulSet(exStatefulSet, &template, -1, "")
		if err != nil {
			return errors.Wrap(err, "Could not generate StatefulSet template for single zone")
		}
		desiredVolumeManagementStatefulSets = append(desiredVolumeManagementStatefulSets, *statefulSet)
	}

	for _, desiredVolumeManagementStatefulSet := range desiredVolumeManagementStatefulSets {

		originalTemplate := exStatefulSet.Spec.Template.DeepCopy()
		// Set the owner of the StatefulSet, so it's garbage collected,
		// and we can find it later
		ctxlog.Info(ctx, "Setting owner for StatefulSet '", desiredVolumeManagementStatefulSet.Name, "' to ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")
		if err := r.setReference(exStatefulSet, &desiredVolumeManagementStatefulSet, r.scheme); err != nil {
			return errors.Wrapf(err, "could not set owner for volumeManagement StatefulSet '%s' to ExtendedStatefulSet '%s' in namespace '%s'", desiredVolumeManagementStatefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
		}

		// Create the StatefulSet
		if err := r.client.Create(ctx, &desiredVolumeManagementStatefulSet); err != nil {
			return errors.Wrapf(err, "could not create volumeManagement StatefulSet '%s' for ExtendedStatefulSet '%s' in namespace '%s'", desiredVolumeManagementStatefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
		}

		ctxlog.Info(ctx, "Created VolumeManagement StatefulSet '", desiredVolumeManagementStatefulSet.Name, "' for ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")

		exStatefulSet.Spec.Template = *originalTemplate
	}
	return nil
}

// generateVolumeManagementSingleStatefulSet creates a volumeManagement single statefulSet per zone
func (r *ReconcileExtendedStatefulSet) generateVolumeManagementSingleStatefulSet(exStatefulSet *essv1a1.ExtendedStatefulSet, template *v1beta2.StatefulSet, zoneIndex int, zoneName string) (*v1beta2.StatefulSet, error) {

	statefulSet := template.DeepCopy()

	// Get the labels and annotations
	labels := statefulSet.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	annotations := statefulSet.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	statefulSetNamePrefix := exStatefulSet.GetName()

	// Update available-zone specified properties
	if zoneIndex >= 0 && len(zoneName) != 0 {
		// Reset name prefix with zoneIndex
		statefulSetNamePrefix = fmt.Sprintf("%s-z%d", exStatefulSet.GetName(), zoneIndex)

		labels[essv1a1.LabelAZIndex] = strconv.Itoa(zoneIndex)
		labels[essv1a1.LabelAZName] = zoneName

		zonesBytes, err := json.Marshal(exStatefulSet.Spec.Zones)
		if err != nil {
			return &v1beta2.StatefulSet{}, errors.Wrapf(err, "Could not marshal zones: '%v'", exStatefulSet.Spec.Zones)
		}
		annotations[essv1a1.AnnotationZones] = string(zonesBytes)

		// Get the pod labels and annotations
		podLabels := statefulSet.Spec.Template.GetLabels()
		if podLabels == nil {
			podLabels = make(map[string]string)
		}
		podLabels[essv1a1.LabelAZIndex] = strconv.Itoa(zoneIndex)
		podLabels[essv1a1.LabelAZName] = zoneName

		podAnnotations := statefulSet.Spec.Template.GetAnnotations()
		if podAnnotations == nil {
			podAnnotations = make(map[string]string)
		}
		podAnnotations[essv1a1.AnnotationZones] = string(zonesBytes)

		statefulSet.Spec.Template.SetLabels(podLabels)
		statefulSet.Spec.Template.SetAnnotations(podAnnotations)

		statefulSet = r.updateAffinity(statefulSet, exStatefulSet.Spec.ZoneNodeLabel, zoneName)
	}

	// Set updated properties
	statefulSet.SetName(fmt.Sprintf("%s-%s", "volume-management", statefulSetNamePrefix))
	statefulSet.SetLabels(labels)
	statefulSet.SetAnnotations(annotations)

	statefulSet.Spec.PodManagementPolicy = v1beta2.ParallelPodManagement

	// Generate dummy volumeMounts
	// "volumeClaimTemplates: [...] Every claim in this list must have at least one matching (by name) volumeMount in one container in the template."
	// See https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.16/#statefulsetspec-v1-apps
	volumeMounts := make([]corev1.VolumeMount, 0, len(statefulSet.Spec.VolumeClaimTemplates))
	for _, pvc := range statefulSet.Spec.VolumeClaimTemplates {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      pvc.Name,
			MountPath: path.Join("/mnt", pvc.Name),
		})
	}

	statefulSet.Spec.Template.Spec.InitContainers = []corev1.Container{}
	statefulSet.Spec.Template.Spec.Containers = []corev1.Container{{
		Name:            "volume-management",
		VolumeMounts:    volumeMounts,
		Image:           converter.GetOperatorDockerImage(),
		ImagePullPolicy: converter.GetOperatorImagePullPolicy(),
		Command: []string{
			"ruby",
			"-e",
			"sleep()",
		},
	}}

	return statefulSet, nil
}

// isVolumeManagementStatefulSetInitialized checks if all the statefulSet pods are initialized
func (r *ReconcileStatefulSetCleanup) isVolumeManagementStatefulSetInitialized(ctx context.Context, statefulSet *v1beta2.StatefulSet) (bool, error) {
	pod := &corev1.Pod{}

	replicaCount := int(*statefulSet.Spec.Replicas)
	for count := 0; count < replicaCount; count++ {
		podName := fmt.Sprintf("%s-%d", statefulSet.Name, count)
		key := types.NamespacedName{Namespace: statefulSet.GetNamespace(), Name: podName}
		err := r.client.Get(ctx, key, pod)
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for pod by name: %v", podName)
		}
		_, condition := podutil.GetPodCondition(&pod.Status, corev1.PodInitialized)
		if condition == nil || condition.Status != corev1.ConditionTrue {
			return false, nil
		}
	}
	return true, nil
}

// deleteVolumeManagementStatefulSet deletes the statefulSet created for volume management
func (r *ReconcileStatefulSetCleanup) deleteVolumeManagementStatefulSet(ctx context.Context, extendedstatefulset *essv1a1.ExtendedStatefulSet) error {

	statefulSets, err := listStatefulSetsFromInformer(ctx, r.client, extendedstatefulset)
	if err != nil {
		return err
	}
	for index := range statefulSets {
		if isVolumeManagementStatefulSet(statefulSets[index].Name) {
			ok, err := r.isVolumeManagementStatefulSetInitialized(ctx, &statefulSets[index])
			if err != nil {
				return err
			}
			if ok {
				ctxlog.Info(ctx, "Deleting volumeManagement statefulSet ", statefulSets[index].Name, " owned by ExtendedStatefulSet ", extendedstatefulset.Name, " in namespace ", extendedstatefulset.Namespace, ".")
				err = r.client.Delete(ctx, &statefulSets[index], client.PropagationPolicy(metav1.DeletePropagationBackground))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// isVolumeManagementStatefulSet checks if it is a statefulSet created by the volume-management for its volume management purpose
func isVolumeManagementStatefulSet(statefulSetName string) bool {
	return strings.HasPrefix(statefulSetName, "volume-management")
}
