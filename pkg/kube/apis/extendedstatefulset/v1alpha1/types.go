package v1alpha1

import (
	"fmt"

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
	// AnnotationVersion is the annotation key for the StatefulSet version
	AnnotationVersion = fmt.Sprintf("%s/version", apis.GroupName)
	// AnnotationZones is an array of all zones
	AnnotationZones = fmt.Sprintf("%s/zones", apis.GroupName)
	// LabelAZIndex is the index of available zone
	LabelAZIndex = fmt.Sprintf("%s/az-index", apis.GroupName)
	// LabelAZName is the name of available zone
	LabelAZName = fmt.Sprintf("%s/az-name", apis.GroupName)
	// LabelPodOrdinal is the index of pod ordinal
	LabelPodOrdinal = fmt.Sprintf("%s/pod-ordinal", apis.GroupName)
	// LabelEStsName is the name of the ExtendedStatefulSet owns this resource
	LabelEStsName = fmt.Sprintf("%s/extendedstatefulset-name", apis.GroupName)
)

// ExtendedStatefulSetSpec defines the desired state of ExtendedStatefulSet
type ExtendedStatefulSetSpec struct {
	// Indicates whether to update Pods in the StatefulSet when an env value or mount changes
	UpdateOnConfigChange bool `json:"updateOnConfigChange"`

	// Indicates the node label that a node locates
	ZoneNodeLabel string `json:"zoneNodeLabel,omitempty"`

	// Indicates the availability zones that the ExtendedStatefulSet needs to span
	Zones []string `json:"zones,omitempty"`

	// Defines a regular StatefulSet template
	Template v1beta2.StatefulSet `json:"template"`
}

// ExtendedStatefulSetStatus defines the observed state of ExtendedStatefulSet
type ExtendedStatefulSetStatus struct {
	// Timestamp for the last reconcile
	LastReconcile *metav1.Time `json:"lastReconcile"`
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
