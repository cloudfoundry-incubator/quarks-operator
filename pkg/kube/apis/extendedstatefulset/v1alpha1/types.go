package v1alpha1

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

var (
	// AnnotationStatefulSetSHA1 is the annotation key for the StatefulSet SHA1
	AnnotationStatefulSetSHA1 = fmt.Sprintf("%s/statefulsetsha1", apis.GroupName)
	// AnnotationConfigSHA1 is the annotation key for the StatefulSet Config(ConfigMap/Secret) SHA1
	AnnotationConfigSHA1 = fmt.Sprintf("%s/configsha1", apis.GroupName)
	// AnnotationVersion is the annotation key for the StatefulSet version
	AnnotationVersion = fmt.Sprintf("%s/version", apis.GroupName)

	// FinalizerString is the finalizer added to objects
	FinalizerString = fmt.Sprintf("%s/finalizer", apis.GroupName)
)

// Object is used as a helper interface when passing Kubernetes resources
// between methods.
// All Kubernetes resources should implement both of these interfaces
type Object interface {
	runtime.Object
	metav1.Object
}

// ExtendedStatefulSetSpec defines the desired state of ExtendedStatefulSet
type ExtendedStatefulSetSpec struct {
	Template v1beta1.StatefulSet `json:"template"`
}

// ExtendedStatefulSetStatus defines the observed state of ExtendedStatefulSet
type ExtendedStatefulSetStatus struct {
	// Map of version number keys and values that keeps track of if version is running
	Versions map[int]bool `json:"versions"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedStatefulSet is the Schema for the extendedstatefulset API
// +k8s:openapi-gen=true
type ExtendedStatefulSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExtendedStatefulSetSpec   `json:"spec,omitempty"`
	Status ExtendedStatefulSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedStatefulSetList contains a list of ExtendedStatefulSet
type ExtendedStatefulSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExtendedStatefulSet `json:"items"`
}

// CalculateDesiredStatefulSetName calculates the name of the StatefulSet to be managed
func (e *ExtendedStatefulSet) CalculateDesiredStatefulSetName(actualStatefulSet *v1beta1.StatefulSet) (string, error) {
	version, err := e.DesiredVersion(actualStatefulSet)
	if err != nil {
		return "", err
	}

	// <extendedstatefulset.name>-v<version>
	return fmt.Sprintf("%s-v%d", e.GetName(), version), nil
}

// DesiredVersion calculates the desired version of the StatefulSet
// If the template of the StatefulSet has changed, the desired version is incremented
func (e *ExtendedStatefulSet) DesiredVersion(actualStatefulSet *v1beta1.StatefulSet) (int, error) {
	strVersion, ok := actualStatefulSet.Annotations[AnnotationVersion]
	if !ok {
		strVersion = "0"
	}
	oldsha, _ := actualStatefulSet.Annotations[AnnotationStatefulSetSHA1]

	version, err := strconv.Atoi(strVersion)
	if err != nil {
		return 0, errors.Wrap(err, "Version annotation is not an int!")
	}

	currentsha, err := e.CalculateStatefulSetSHA1()
	if err != nil {
		return 0, err
	}

	if currentsha != oldsha {
		version++
	}

	return version, nil
}

// CalculateStatefulSetSHA1 calculates the SHA1 of the JSON representation of the
// StatefulSet template
func (e *ExtendedStatefulSet) CalculateStatefulSetSHA1() (string, error) {
	data, err := json.Marshal(e.Spec.Template)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha1.Sum(data)), nil
}

// GetMaxAvailableVersion gets the greatest available version owned by the ExtendedStatefulSet
func (e *ExtendedStatefulSet) GetMaxAvailableVersion(versions map[int]bool) int {
	maxAvailableVersion := 0

	for version, available := range versions {
		if available && version > maxAvailableVersion {
			maxAvailableVersion = version
		}
	}
	return maxAvailableVersion
}

// AddFinalizer adds the finalizer item to the ExtendedStatefulSet
func (e *ExtendedStatefulSet) AddFinalizer() {
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
func (e *ExtendedStatefulSet) RemoveFinalizer() {
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

// ToBeDeleted checks whether this ExtendedStatefulSet has been marked for deletion
func (e *ExtendedStatefulSet) ToBeDeleted() bool {
	// IsZero means that the object hasn't been marked for deletion
	return !e.GetDeletionTimestamp().IsZero()
}
