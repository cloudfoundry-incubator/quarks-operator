package statefulset

import (
	"context"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	podutil "code.cloudfoundry.org/quarks-utils/pkg/pod"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

// ConfigureStatefulSetForRollout configures a stateful set for canarying and rollout
func ConfigureStatefulSetForRollout(statefulSet *appsv1.StatefulSet) {
	statefulSet.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	//the canary rollout is for now directly started, the might move to a webhook instead
	statefulSet.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{
		Partition: pointers.Int32(util.MinInt32(*statefulSet.Spec.Replicas, statefulSet.Status.Replicas)),
	}
	statefulSet.Annotations[AnnotationCanaryRollout] = rolloutStatePending
	statefulSet.Annotations[AnnotationUpdateStartTime] = strconv.FormatInt(time.Now().Unix(), 10)
}

// ConfigureStatefulSetForInitialRollout initially configures a stateful set for canarying and rollout
func ConfigureStatefulSetForInitialRollout(statefulSet *appsv1.StatefulSet) {
	statefulSet.Spec.UpdateStrategy.Type = appsv1.RollingUpdateStatefulSetStrategyType
	//the canary rollout is for now directly started, the might move to a webhook instead
	statefulSet.Spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{
		Partition: pointers.Int32(0),
	}
	statefulSet.Annotations[AnnotationCanaryRollout] = rolloutStateCanaryUpscale
	statefulSet.Annotations[AnnotationUpdateStartTime] = strconv.FormatInt(time.Now().Unix(), 10)
}

// FilterLabels filters out labels, that are not suitable for StatefulSet updates
func FilterLabels(labels map[string]string) map[string]string {

	statefulSetLabels := make(map[string]string)
	for key, value := range labels {
		if key != bdv1.LabelDeploymentVersion {
			statefulSetLabels[key] = value
		}
	}
	return statefulSetLabels
}

// CleanupNonReadyPod deletes all pods, that are not ready
func CleanupNonReadyPod(ctx context.Context, client crc.Client, statefulSet *appsv1.StatefulSet, index int32) error {
	ctxlog.Debug(ctx, "Cleaning up non ready pod for StatefulSet ", statefulSet.Namespace, "/", statefulSet.Name, "-", index)
	pod, ready, err := getPodWithIndex(ctx, client, statefulSet, index)
	if err != nil {
		return err
	}
	if ready || pod == nil {
		return nil
	}
	ctxlog.Debug(ctx, "Deleting pod ", pod.Name)
	if err = client.Delete(ctx, pod); err != nil {
		ctxlog.Error(ctx, "Error deleting non-ready pod ", err)
	}
	return err
}

// getPodWithIndex returns a pod for a given statefulset and index
func getPodWithIndex(ctx context.Context, client crc.Client, statefulSet *appsv1.StatefulSet, index int32) (*corev1.Pod, bool, error) {
	var pod corev1.Pod
	podName := fmt.Sprintf("%s-%d", statefulSet.Name, index)
	err := client.Get(ctx, crc.ObjectKey{Name: podName, Namespace: statefulSet.Namespace}, &pod)
	if err != nil {
		if crc.IgnoreNotFound(err) == nil {
			ctxlog.Error(ctx, "Pods ", podName, " belonging to StatefulSet not found", statefulSet.Name, ":", err)
			return nil, false, nil
		}
		return nil, false, err
	}
	return &pod, podutil.IsPodReady(&pod), nil
}
