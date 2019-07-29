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

// InWindow returns true if the given time is in the meltdown window
func InWindow(now time.Time, duration time.Duration, annotations map[string]string) bool {
	if ts, ok := annotations[AnnotationLastReconcile]; ok {
		if windowStart, err := time.Parse(time.RFC3339, ts); err == nil {
			if now.Before(windowStart) {
				// should not happen
				return false
			}
			windowEnd := windowStart.Add(duration)
			return now.Before(windowEnd)
		}
	}
	return false
}

// SetLastReconcile annotation in object meta to the given time
func SetLastReconcile(objectMeta *metav1.ObjectMeta, now time.Time) {
	metav1.SetMetaDataAnnotation(objectMeta, AnnotationLastReconcile, now.Format(time.RFC3339))
}
