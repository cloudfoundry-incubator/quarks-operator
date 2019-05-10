package extendedjob

import (
	"fmt"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
)

// InferPodState determines our pod state
//	   ready (p:Running,c:[Initialized Ready ContainersReady PodScheduled])
//	   created (p:Pending,c:[Initialized Ready ContainersReady PodScheduled])
//	   notready (p:Pending,c:[PodScheduled])
//	   notready (p:Pending,c:[])
//	   deleted (p:Running,c:[Initialized PodScheduled],deletionGracePeriodSeconds == 0)
func InferPodState(pod corev1.Pod) ejv1.PodState {

	// if deletionGracePeriodSeconds is zero, it is deletestate
	if pod.DeletionGracePeriodSeconds != nil {
		if *pod.DeletionGracePeriodSeconds == 0 {
			return ejv1.PodStateDeleted
		}
	}

	// reconcile triggers update with a running pod for deletion, too
	if pod.Status.Phase == "Running" {
		return ejv1.PodStateReady
	}

	if pod.Status.Phase == "Pending" {
		scheduled, _ := podutil.GetPodCondition(&pod.Status, corev1.PodScheduled)
		ready, _ := podutil.GetPodCondition(&pod.Status, corev1.PodReady)
		if scheduled > -1 && ready > -1 {
			return ejv1.PodStateCreated
		}
		return ejv1.PodStateNotReady
	}

	return ejv1.PodStateUnknown
}

// PodStatusString gives a summary of the pods state
func PodStatusString(pod corev1.Pod) string {
	conditions := []string{}
	for _, c := range pod.Status.Conditions {
		conditions = append(conditions, fmt.Sprintf("%s", c.Type))
	}
	return fmt.Sprintf("phase=%s conditions=%s)", pod.Status.Phase, conditions)
}
