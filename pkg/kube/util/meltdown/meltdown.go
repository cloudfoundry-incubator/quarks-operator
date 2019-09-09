package meltdown

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

// AnnotationLastReconcile is the key name of the timestamp for the last
// successful reconcile
var AnnotationLastReconcile = fmt.Sprintf("%s/last-reconcile", apis.GroupName)

// Window represents a time window starting at start and ending at duration
type Window struct {
	Start    time.Time
	Duration time.Duration
}

// Contains returns true if the given time is in the specified time window
func (w Window) Contains(now time.Time) bool {
	if now.Before(w.Start) {
		return false
	}
	windowEnd := w.Start.Add(w.Duration)
	return now.Before(windowEnd)
}

// NewWindow returns a new time window starting at lastReconcile, ending after duration
func NewWindow(duration time.Duration, lastReconcile *metav1.Time) Window {
	if lastReconcile != nil {
		start := lastReconcile.Time
		return Window{Start: start, Duration: duration}
	}
	return Window{}
}

// NewAnnotationWindow returns a window starting at lastReconcile contained in
// annotations, ending after duration
func NewAnnotationWindow(duration time.Duration, annotations map[string]string) Window {
	if ts, ok := annotations[AnnotationLastReconcile]; ok {
		if start, err := time.Parse(time.RFC3339, ts); err == nil {
			return Window{Start: start, Duration: duration}
		}
	}
	return Window{}
}

// SetLastReconcile annotation in object meta to the given time
func SetLastReconcile(objectMeta *metav1.ObjectMeta, now time.Time) {
	metav1.SetMetaDataAnnotation(objectMeta, AnnotationLastReconcile, now.Format(time.RFC3339))
}
