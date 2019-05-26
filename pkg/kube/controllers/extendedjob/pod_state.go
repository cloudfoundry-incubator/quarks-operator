package extendedjob

import (
	corev1 "k8s.io/api/core/v1"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
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

	if pod.Status.Phase == "Running" {
		if podutil.IsPodReady(&pod) && pod.DeletionTimestamp == nil {
			return ejv1.PodStateReady
		}
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
