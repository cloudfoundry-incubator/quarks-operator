package extendedstatefulset

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	essv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
)

// alterVolumeManagementStatefulSet creates the volumemanagement statefulset for persistent volume claim creation
func (r *ReconcileExtendedStatefulSet) alterVolumeManagementStatefulSet(ctx context.Context, actualVersion int, desiredVersion int, exStatefulSet *essv1a1.ExtendedStatefulSet, actualStatefulSet *v1beta2.StatefulSet) error {

	// Create volumemanagement statefulset if it is the first time extendedstatefulset is created
	if actualVersion == 0 && actualVersion != desiredVersion {
		err := r.createVolumeManagementStatefulSets(ctx, exStatefulSet, actualStatefulSet)
		if err != nil {
			return errors.Wrapf(err, "Creation of volumemanagement statefulset failed.")
		}
	} else {
		replicaDifference := r.getReplicaDifference(exStatefulSet, actualStatefulSet)
		if replicaDifference > 0 {
			err := r.createVolumeManagementStatefulSets(ctx, exStatefulSet, actualStatefulSet)
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

// createVolumeManagementStatefulSet creates a volumemanagement statefulset
func (r *ReconcileExtendedStatefulSet) createVolumeManagementStatefulSets(ctx context.Context, exStatefulSet *essv1a1.ExtendedStatefulSet, statefulSet *v1beta2.StatefulSet) error {

	var desiredVolumeManagementStatefulSets []v1beta2.StatefulSet

	template := exStatefulSet.Spec.Template
	template.SetName(fmt.Sprintf("%s", "volumemanagement"))

	// Place the StatefulSet in the same namespace as the ExtendedStatefulSet
	template.SetNamespace(exStatefulSet.Namespace)

	if exStatefulSet.Spec.ZoneNodeLabel == "" {
		exStatefulSet.Spec.ZoneNodeLabel = essv1a1.DefaultZoneNodeLabel
	}

	if len(exStatefulSet.Spec.Zones) > 0 {
		for zoneIndex, zoneName := range exStatefulSet.Spec.Zones {
			statefulSet, err := r.generateVolumeManagementSingleStatefulSet(exStatefulSet, &template, zoneIndex, zoneName)
			if err != nil {
				return errors.Wrapf(err, "Could not generate volumemanagement StatefulSet template for AZ '%d/%s'", zoneIndex, zoneName)
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
			return errors.Wrapf(err, "could not set owner for volumemanagement StatefulSet '%s' to ExtendedStatefulSet '%s' in namespace '%s'", desiredVolumeManagementStatefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
		}

		// Create the StatefulSet
		if err := r.client.Create(ctx, &desiredVolumeManagementStatefulSet); err != nil {
			return errors.Wrapf(err, "could not create volumemanagement StatefulSet '%s' for ExtendedStatefulSet '%s' in namespace '%s'", desiredVolumeManagementStatefulSet.Name, exStatefulSet.Name, exStatefulSet.Namespace)
		}

		ctxlog.Info(ctx, "Created VolumeManagement StatefulSet '", desiredVolumeManagementStatefulSet.Name, "' for ExtendedStatefulSet '", exStatefulSet.Name, "' in namespace '", exStatefulSet.Namespace, "'.")

		exStatefulSet.Spec.Template = *originalTemplate
	}
	return nil
}

// generateVolumeManagementSingleStatefulSet creates a volumemanagement single statefulset per zone
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

		statefulSet = r.updateAffinity(statefulSet, exStatefulSet.Spec.ZoneNodeLabel, zoneIndex, zoneName)
	}

	// Set updated properties
	statefulSet.SetName(fmt.Sprintf("%s-%s", "volumemanagement", statefulSetNamePrefix))
	statefulSet.SetLabels(labels)
	statefulSet.SetAnnotations(annotations)

	return statefulSet, nil
}

// isVolumeManagementStatefulSetReady checks if all the statefulset pods are ready
func (r *ReconcileExtendedStatefulSet) isVolumeManagementStatefulSetReady(ctx context.Context, statefulSet *v1beta2.StatefulSet) (bool, error) {
	pod := &corev1.Pod{}

	replicaCount := int(*statefulSet.Spec.Replicas)
	for count := 0; count < replicaCount; count++ {
		podName := fmt.Sprintf("%s-%d", statefulSet.Name, count)
		key := types.NamespacedName{Namespace: statefulSet.GetNamespace(), Name: podName}
		err := r.client.Get(ctx, key, pod)
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for pod by name: %v", podName)
		}
		if !podutil.IsPodReady(pod) {
			return false, nil
		}
	}
	return true, nil
}

// isVolumeManagementStatefulSet checks if it is a statefulset created by the volumemanagement for its volume management purpose
func (r *ReconcileExtendedStatefulSet) isVolumeManagementStatefulSet(statefulSetName string) bool {
	statefulSetNamePrefix := strings.Split(statefulSetName, "-")[0]
	if statefulSetNamePrefix == "volumemanagement" {
		return true
	}
	return false
}

// deleteVolumeManagementStatefulSet deletes the statefulset created for volume management
func (r *ReconcileExtendedStatefulSet) deleteVolumeManagementStatefulSet(ctx context.Context, extendedstatefulset *essv1a1.ExtendedStatefulSet) error {

	statefulSets, err := r.listStatefulSets(ctx, extendedstatefulset)
	if err != nil {
		return err
	}
	for index := range statefulSets {
		if r.isVolumeManagementStatefulSet(statefulSets[index].Name) {
			ok, err := r.isVolumeManagementStatefulSetReady(ctx, &statefulSets[index])
			if err != nil {
				return err
			}
			if ok {
				ctxlog.Info(ctx, "Deleting volumemanagement statefulset ", statefulSets[index].Name, " owned by ExtendedStatefulSet ", extendedstatefulset.Name, " in namespace ", extendedstatefulset.Namespace, ".")
				err = r.client.Delete(ctx, &statefulSets[index], client.PropagationPolicy(metav1.DeletePropagationBackground))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}
