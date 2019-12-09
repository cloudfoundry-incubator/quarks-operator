package statefulset

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	podutil "code.cloudfoundry.org/quarks-utils/pkg/pod"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"
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

// FilterLabels filters out labels, that are not suitable for StatefulSet updates
func FilterLabels(labels map[string]string) map[string]string {

	statefulSetLabels := make(map[string]string)
	for key, value := range labels {
		if key != manifest.LabelDeploymentVersion {
			statefulSetLabels[key] = value
		}
	}
	return statefulSetLabels
}

//ComputeAnnotations computes annotations from the instance group
func ComputeAnnotations(ig *manifest.InstanceGroup) (map[string]string, error) {
	statefulSetAnnotations := ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations
	if statefulSetAnnotations == nil {
		statefulSetAnnotations = make(map[string]string)
	}
	if ig.Update == nil {
		return statefulSetAnnotations, nil
	}

	canaryWatchTime, err := ExtractWatchTime(ig.Update.CanaryWatchTime, "canary_watch_time")
	if err != nil {
		return nil, err
	}
	if canaryWatchTime != "" {
		statefulSetAnnotations[AnnotationCanaryWatchTime] = canaryWatchTime
	}

	updateWatchTime, err := ExtractWatchTime(ig.Update.UpdateWatchTime, "update_watch_time")
	if err != nil {
		return nil, err
	}

	if updateWatchTime != "" {
		statefulSetAnnotations[AnnotationUpdateWatchTime] = updateWatchTime
	}

	return statefulSetAnnotations, nil
}

//ExtractWatchTime computes the watch time from a range or an absolute value
func ExtractWatchTime(rawWatchTime string, field string) (string, error) {
	if rawWatchTime == "" {
		return "", nil
	}

	rangeRegex := regexp.MustCompile(`^\s*(\d+)\s*-\s*(\d+)\s*$`) // https://github.com/cloudfoundry/bosh/blob/914edca5278b994df7d91620c4f55f1c6665f81c/src/bosh-director/lib/bosh/director/deployment_plan/update_config.rb#L128
	if matches := rangeRegex.FindStringSubmatch(rawWatchTime); len(matches) > 0 {
		// Ignore the lower boundary, because the API-Server triggers reconciles
		return matches[2], nil
	}
	absoluteRegex := regexp.MustCompile(`^\s*(\d+)\s*$`) // https://github.com/cloudfoundry/bosh/blob/914edca5278b994df7d91620c4f55f1c6665f81c/src/bosh-director/lib/bosh/director/deployment_plan/update_config.rb#L130
	if matches := absoluteRegex.FindStringSubmatch(rawWatchTime); len(matches) > 0 {
		return matches[1], nil
	}
	return "", fmt.Errorf("invalid %s", field)
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
	return client.Delete(ctx, pod)
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
