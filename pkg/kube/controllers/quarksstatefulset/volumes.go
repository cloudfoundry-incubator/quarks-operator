package quarksstatefulset

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

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	podutil "code.cloudfoundry.org/quarks-utils/pkg/pod"
)

// alterVolumeManagementStatefulSet creates the volumeManagement statefulSet for persistent volume claim creation.
func (r *ReconcileQuarksStatefulSet) alterVolumeManagementStatefulSet(ctx context.Context, currentVersion int, desiredVersion int, qStatefulSet *qstsv1a1.QuarksStatefulSet, currentStatefulSet *v1beta2.StatefulSet) error {
	// Create volumeManagement statefulSet if it is the first time quarksStatefulSet is created.
	if currentVersion == 0 && currentVersion != desiredVersion {
		err := r.createVolumeManagementStatefulSets(ctx, qStatefulSet, currentStatefulSet)
		if err != nil {
			return errors.Wrapf(err, "Creation of volumeManagement statefulSets failed.")
		}
	} else {
		replicaDifference := r.getReplicaDifference(qStatefulSet, currentStatefulSet)
		if replicaDifference > 0 {
			err := r.createVolumeManagementStatefulSets(ctx, qStatefulSet, currentStatefulSet)
			if err != nil {
				return errors.Wrapf(err, "Creation of VolumeManagement StatefulSets failed")
			}
		}
	}
	return nil
}

// getReplicaDifference calculates the difference between replica count
func (r *ReconcileQuarksStatefulSet) getReplicaDifference(qStatefulSet *qstsv1a1.QuarksStatefulSet, statefulSet *v1beta2.StatefulSet) int {
	return int(*qStatefulSet.Spec.Template.Spec.Replicas) - int(*statefulSet.Spec.Replicas)
}

// createVolumeManagementStatefulSet creates a volumeManagement statefulSet
func (r *ReconcileQuarksStatefulSet) createVolumeManagementStatefulSets(ctx context.Context, qStatefulSet *qstsv1a1.QuarksStatefulSet, statefulSet *v1beta2.StatefulSet) error {

	var desiredVolumeManagementStatefulSets []v1beta2.StatefulSet

	template := qStatefulSet.Spec.Template
	template.SetName("volume-management")

	// Place the StatefulSet in the same namespace as the QuarksStatefulSet
	template.SetNamespace(qStatefulSet.Namespace)

	if qStatefulSet.Spec.ZoneNodeLabel == "" {
		qStatefulSet.Spec.ZoneNodeLabel = qstsv1a1.DefaultZoneNodeLabel
	}

	if len(qStatefulSet.Spec.Zones) > 0 {
		for zoneIndex, zoneName := range qStatefulSet.Spec.Zones {
			statefulSet, err := r.generateVolumeManagementSingleStatefulSet(qStatefulSet, &template, zoneIndex, zoneName)
			if err != nil {
				return errors.Wrapf(err, "Could not generate volumeManagement StatefulSet template for AZ '%d/%s'", zoneIndex, zoneName)
			}
			desiredVolumeManagementStatefulSets = append(desiredVolumeManagementStatefulSets, *statefulSet)
		}
	} else {
		statefulSet, err := r.generateVolumeManagementSingleStatefulSet(qStatefulSet, &template, -1, "")
		if err != nil {
			return errors.Wrap(err, "Could not generate StatefulSet template for single zone")
		}
		desiredVolumeManagementStatefulSets = append(desiredVolumeManagementStatefulSets, *statefulSet)
	}

	for _, desiredVolumeManagementStatefulSet := range desiredVolumeManagementStatefulSets {

		originalTemplate := qStatefulSet.Spec.Template.DeepCopy()
		// Set the owner of the StatefulSet, so it's garbage collected,
		// and we can find it later
		ctxlog.Info(ctx, "Setting owner for StatefulSet '", desiredVolumeManagementStatefulSet.Name, "' to QuarksStatefulSet '", qStatefulSet.Name, "' in namespace '", qStatefulSet.Namespace, "'.")
		if err := r.setReference(qStatefulSet, &desiredVolumeManagementStatefulSet, r.scheme); err != nil {
			return errors.Wrapf(err, "could not set owner for volumeManagement StatefulSet '%s' to QuarksStatefulSet '%s' in namespace '%s'", desiredVolumeManagementStatefulSet.Name, qStatefulSet.Name, qStatefulSet.Namespace)
		}

		// Create the StatefulSet
		if err := r.client.Create(ctx, &desiredVolumeManagementStatefulSet); err != nil {
			return errors.Wrapf(err, "could not create volumeManagement StatefulSet '%s' for QuarksStatefulSet '%s' in namespace '%s'", desiredVolumeManagementStatefulSet.Name, qStatefulSet.Name, qStatefulSet.Namespace)
		}

		ctxlog.Info(ctx, "Created VolumeManagement StatefulSet '", desiredVolumeManagementStatefulSet.Name, "' for QuarksStatefulSet '", qStatefulSet.Name, "' in namespace '", qStatefulSet.Namespace, "'.")

		qStatefulSet.Spec.Template = *originalTemplate
	}
	return nil
}

// generateVolumeManagementSingleStatefulSet creates a volumeManagement single statefulSet per zone
func (r *ReconcileQuarksStatefulSet) generateVolumeManagementSingleStatefulSet(qStatefulSet *qstsv1a1.QuarksStatefulSet, template *v1beta2.StatefulSet, zoneIndex int, zoneName string) (*v1beta2.StatefulSet, error) {

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

	statefulSetNamePrefix := qStatefulSet.GetName()

	// Update available-zone specified properties
	if zoneIndex >= 0 && len(zoneName) != 0 {
		// Reset name prefix with zoneIndex
		statefulSetNamePrefix = fmt.Sprintf("%s-z%d", qStatefulSet.GetName(), zoneIndex)

		labels[qstsv1a1.LabelAZIndex] = strconv.Itoa(zoneIndex)
		labels[qstsv1a1.LabelAZName] = zoneName

		zonesBytes, err := json.Marshal(qStatefulSet.Spec.Zones)
		if err != nil {
			return &v1beta2.StatefulSet{}, errors.Wrapf(err, "Could not marshal zones: '%v'", qStatefulSet.Spec.Zones)
		}
		annotations[qstsv1a1.AnnotationZones] = string(zonesBytes)

		// Get the pod labels and annotations
		podLabels := statefulSet.Spec.Template.GetLabels()
		if podLabels == nil {
			podLabels = make(map[string]string)
		}
		podLabels[qstsv1a1.LabelAZIndex] = strconv.Itoa(zoneIndex)
		podLabels[qstsv1a1.LabelAZName] = zoneName

		podAnnotations := statefulSet.Spec.Template.GetAnnotations()
		if podAnnotations == nil {
			podAnnotations = make(map[string]string)
		}
		podAnnotations[qstsv1a1.AnnotationZones] = string(zonesBytes)

		statefulSet.Spec.Template.SetLabels(podLabels)
		statefulSet.Spec.Template.SetAnnotations(podAnnotations)

		statefulSet = r.updateAffinity(statefulSet, qStatefulSet.Spec.ZoneNodeLabel, zoneName)
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
func (r *ReconcileStatefulSetCleanup) deleteVolumeManagementStatefulSet(ctx context.Context, qStatefulSet *qstsv1a1.QuarksStatefulSet) error {

	statefulSets, err := listStatefulSetsFromInformer(ctx, r.client, qStatefulSet)
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
				ctxlog.Info(ctx, "Deleting volumeManagement statefulSet ", statefulSets[index].Name, " owned by QuarksStatefulSet ", qStatefulSet.Name, " in namespace ", qStatefulSet.Namespace, ".")
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
