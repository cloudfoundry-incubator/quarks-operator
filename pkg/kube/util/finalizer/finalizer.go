package finalizer

import (
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// AnnotationFinalizer is the finalizer added to objects
	AnnotationFinalizer = fmt.Sprintf("%s/finalizer", apis.GroupName)
)

// AddFinalizer adds the finalizer item to the ExtendedStatefulSet
func AddFinalizer(e metav1.Object) {
	finalizers := e.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == AnnotationFinalizer {
			// ExtendedStatefulSet already contains the finalizer
			return
		}
	}

	// ExtendedStatefulSet doesn't contain the finalizer, so add it
	finalizers = append(finalizers, AnnotationFinalizer)
	e.SetFinalizers(finalizers)
}

// RemoveFinalizer removes the finalizer item from the ExtendedStatefulSet
func RemoveFinalizer(e metav1.Object) {
	finalizers := e.GetFinalizers()

	// Remove any that match the finalizerString
	newFinalizers := []string{}
	for _, finalizer := range finalizers {
		if finalizer != AnnotationFinalizer {
			newFinalizers = append(newFinalizers, finalizer)
		}
	}

	// Update the object's finalizers
	e.SetFinalizers(newFinalizers)
}

// HasFinalizer checks the finalizer item from the ExtendedStatefulSet
func HasFinalizer(e metav1.Object) bool {
	finalizers := e.GetFinalizers()

	for _, finalizer := range finalizers {
		if finalizer == AnnotationFinalizer {
			return true
		}
	}

	return false
}
