package extendedjob

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodEvent is an event with it's corresponding/involved Pod
type PodEvent struct {
	corev1.Event
	Pod *corev1.Pod
}

func (pe PodEvent) podName() string {
	return pe.Pod.Name
}

// isOld returns true if pod timestamp for job is newer than event timestamp
func (pe PodEvent) isOld(name string) bool {
	tsName := jobTimestampName(name)
	annotations := pe.Pod.Annotations
	if ts, ok := annotations[tsName]; ok {

		n, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return false
		}

		podStamp := time.Unix(n, 0)
		eventStamp := pe.Event.LastTimestamp.Time
		return eventStamp.Before(podStamp)
	}
	return false
}

// setTimestamp sets the timestamp on a pod to the current time
func (pe PodEvent) setTimestamp(client client.Client, exJobName string) error {
	tsName := jobTimestampName(exJobName)
	pod := pe.Pod
	// need to reload before update, can't update outdated pod
	name := types.NamespacedName{
		Name:      pod.Name,
		Namespace: pod.Namespace,
	}
	err := client.Get(context.TODO(), name, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return errors.Wrap(err, "pod not found")
		}
		return err
	}

	annotations := pod.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	ts := time.Now().Unix()
	annotations[tsName] = strconv.FormatInt(ts, 10)
	pod.SetAnnotations(annotations)
	err = client.Update(context.TODO(), pod)
	if err != nil {
		delete(annotations, tsName)
		pod.SetAnnotations(annotations)
	}
	return err
}

func jobTimestampName(name string) string {
	return fmt.Sprintf("job-ts-%s", name)
}
