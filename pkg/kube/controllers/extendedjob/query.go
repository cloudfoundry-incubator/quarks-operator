package extendedjob

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ Query = &QueryImpl{}

// Backlog defines the maximal minutes passed for pod events we take into consideration
const Backlog = -30 * time.Minute

// PodEvent is an event with it's corresponding/involved Pod
type PodEvent struct {
	corev1.Event
	Pod corev1.Pod
}

// Query for events involving pods and filter them
type Query interface {
	RecentPodEvents() ([]corev1.Event, error)
	FetchPods([]corev1.Event) ([]PodEvent, error)
	Match(v1alpha1.ExtendedJob, []PodEvent) []PodEvent
}

// NewQuery returns a new Query struct
func NewQuery(c client.Client) *QueryImpl {
	return &QueryImpl{client: c}
}

// QueryImpl implements the query interface
type QueryImpl struct {
	client client.Client
}

// RecentPodEvents returns all events involving pods from the past
func (q *QueryImpl) RecentPodEvents() ([]corev1.Event, error) {
	obj := &corev1.EventList{}
	sel := fields.Set{"involvedObject.kind": "Pod"}.AsSelector()
	err := q.client.List(context.TODO(), &client.ListOptions{FieldSelector: sel}, obj)
	if err != nil {
		return obj.Items, err
	}

	now := time.Now()
	items := []corev1.Event{}
	for _, ev := range obj.Items {
		if ev.LastTimestamp.Time.After(now.Add(Backlog)) {
			items = append(items, ev)
		}
	}

	return items, nil
}

// FetchPods returns all events with their corresponding pod
// It's ok if pods from events no longer exist, since events might be very old.
func (q *QueryImpl) FetchPods(events []corev1.Event) ([]PodEvent, error) {
	podEvents := []PodEvent{}
	for _, ev := range events {
		name := types.NamespacedName{Name: ev.InvolvedObject.Name, Namespace: ev.InvolvedObject.Namespace}
		pod := &corev1.Pod{}
		err := q.client.Get(context.TODO(), name, pod)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return podEvents, err
		}
		podEvents = append(podEvents, PodEvent{Event: ev, Pod: *pod})
	}
	return podEvents, nil
}

// Match pod against label whitelist from extended job
func (q *QueryImpl) Match(job v1alpha1.ExtendedJob, pods []PodEvent) []PodEvent {
	filtered := []PodEvent{}
	// TODO https://github.com/kubernetes/apimachinery/blob/master/pkg/labels/selector.go
	whitelist := job.Spec.Triggers.Selector.MatchLabels
	for _, pod := range pods {
		if labels.AreLabelsInWhiteList(pod.Pod.Labels, whitelist) {
			filtered = append(filtered, pod)
		}
	}
	return filtered
}
