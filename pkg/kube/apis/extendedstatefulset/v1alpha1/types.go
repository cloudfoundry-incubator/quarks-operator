package v1alpha1

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	"github.com/pkg/errors"

	v1beta1 "k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

var (
	// AnnotationStatefulSetSHA1 is the annotation key for the StatefulSet SHA1
	AnnotationStatefulSetSHA1 = fmt.Sprintf("%s/statefulsetsha1", apis.GroupName)
	// AnnotationVersion is the annotation key for the StatefulSet version
	AnnotationVersion = fmt.Sprintf("%s/version", apis.GroupName)
)

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
