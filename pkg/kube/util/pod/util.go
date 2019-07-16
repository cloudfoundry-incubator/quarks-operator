package pod

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// GetPodStatusString gives a summary of the pods state.
func GetPodStatusString(pod corev1.Pod) string {
	conditions := []string{}
	for _, c := range pod.Status.Conditions {
		conditions = append(conditions, fmt.Sprintf("%s=%v", c.Type, c.Status))
	}
	return fmt.Sprintf("phase=%s conditions=%s)", pod.Status.Phase, conditions)
}

// IsPodReady returns whether the pod status is ready or not.
func IsPodReady(pod *corev1.Pod) bool {
	_, condition := GetPodCondition(&pod.Status, corev1.PodReady)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// GetPodCondition returns:
//   1. The index of the condition, or -1 if not present.
//   2. The PodCondition.
func GetPodCondition(
	status *corev1.PodStatus,
	conditionType corev1.PodConditionType,
) (int, *corev1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

// LookupEnv returns a value for a given key from k8s EnvVar arrays
func LookupEnv(env []corev1.EnvVar, name string) (string, bool) {
	for _, v := range env {
		if v.Name == name {
			return v.Value, true
		}
	}
	return "", false
}
