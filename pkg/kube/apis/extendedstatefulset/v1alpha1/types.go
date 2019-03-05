package v1alpha1

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// DefaultZoneNodeLabel is the default node label for available zones
const DefaultZoneNodeLabel = "failure-domain.beta.kubernetes.io/zone"

var (
	// AnnotationStatefulSetSHA1 is the annotation key for the StatefulSet SHA1
	AnnotationStatefulSetSHA1 = fmt.Sprintf("%s/statefulsetsha1", apis.GroupName)
	// AnnotationConfigSHA1 is the annotation key for the StatefulSet Config(ConfigMap/Secret) SHA1
	AnnotationConfigSHA1 = fmt.Sprintf("%s/configsha1", apis.GroupName)
	// AnnotationVersion is the annotation key for the StatefulSet version
	AnnotationVersion = fmt.Sprintf("%s/version", apis.GroupName)
	// AnnotationZones is an array of all zones
	AnnotationZones = fmt.Sprintf("%s/zones", apis.GroupName)
	// LabelAZIndex is the index of available zone
	LabelAZIndex = fmt.Sprintf("%s/az-index", apis.GroupName)
	// LabelAZName is the name of available zone
	LabelAZName = fmt.Sprintf("%s/az-name", apis.GroupName)
)

// ExtendedStatefulSetSpec defines the desired state of ExtendedStatefulSet
type ExtendedStatefulSetSpec struct {
	// Indicates whether to update Pods in the StatefulSet when an env value or mount changes
	UpdateOnEnvChange bool `json:"updateOnEnvChange"`

	// Indicates the node label that a node locates
	ZoneNodeLabel string `json:"zoneNodeLabel,omitempty"`

	// Indicates the availability zones that the ExtendedStatefulSet needs to span
	Zones []string `json:"zones,omitempty"`

	// Defines a regular StatefulSet template
	Template v1beta2.StatefulSet `json:"template"`
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

// DesiredVersion calculates the desired version of the StatefulSet
// If the template of the StatefulSet has changed, the desired version is incremented
func (e *ExtendedStatefulSet) DesiredVersion(actualStatefulSet *v1beta2.StatefulSet) (int, error) {
	strVersion, ok := actualStatefulSet.Annotations[AnnotationVersion]
	if !ok {
		strVersion = "0"
	}
	oldsha, _ := actualStatefulSet.Annotations[AnnotationStatefulSetSHA1]

	version, err := strconv.Atoi(strVersion)
	if err != nil {
		return 0, errors.Wrap(err, "version annotation is not an int!")
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

// ToBeDeleted checks whether this ExtendedStatefulSet has been marked for deletion
func (e *ExtendedStatefulSet) ToBeDeleted() bool {
	// IsZero means that the object hasn't been marked for deletion
	return !e.GetDeletionTimestamp().IsZero()
}
