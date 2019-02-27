package finalizer

import (
	"fmt"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// FinalizerString is the finalizer added to objects
	FinalizerString = fmt.Sprintf("%s/finalizer", apis.GroupName)
)

// AddFinalizer adds the finalizer item to the ExtendedStatefulSet
func AddFinalizer(e metav1.Object) {
	finalizers := e.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == FinalizerString {
			// ExtendedStatefulSet already contains the finalizer
			return
		}
	}

	// ExtendedStatefulSet doesn't contain the finalizer, so add it
	finalizers = append(finalizers, FinalizerString)
	e.SetFinalizers(finalizers)
}

// RemoveFinalizer removes the finalizer item from the ExtendedStatefulSet
func RemoveFinalizer(e metav1.Object) {
	finalizers := e.GetFinalizers()

	// Remove any that match the finalizerString
	newFinalizers := []string{}
	for _, finalizer := range finalizers {
		if finalizer != FinalizerString {
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
		if finalizer == FinalizerString {
			return true
		}
	}

	return false
}
