package v1alpha1

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
	// LabelQStsName is the name of the QuarksStatefulSet owns this resource
	LabelQStsName = fmt.Sprintf("%s/quarks-statefulset-name", apis.GroupName)
	// LabelActiveContainer is the active container on an active/passive setup
	LabelActiveContainer = fmt.Sprintf("%s/pod-active", apis.GroupName)
)

// QuarksStatefulSetSpec defines the desired state of QuarksStatefulSet
type QuarksStatefulSetSpec struct {
	// Indicates whether to update Pods in the StatefulSet when an env value or mount changes
	UpdateOnConfigChange bool `json:"updateOnConfigChange"`

	// Indicates the node label that a node locates
	ZoneNodeLabel string `json:"zoneNodeLabel,omitempty"`

	// Indicates the availability zones that the QuarksStatefulSet needs to span
	Zones []string `json:"zones,omitempty"`

	// Defines a regular StatefulSet template
	Template appsv1.StatefulSet `json:"template"`

	// Periodic probe for active/passive containers
	// Only an active container will process request from a service
	ActivePassiveProbe map[string]*corev1.Probe `json:"activePassiveProbe,omitempty"`
}

// QuarksStatefulSetStatus defines the observed state of QuarksStatefulSet
type QuarksStatefulSetStatus struct {
	// Timestamp for the last reconcile
	LastReconcile *metav1.Time `json:"lastReconcile"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuarksStatefulSet is the Schema for the QuarksStatefulSet API
// +k8s:openapi-gen=true
type QuarksStatefulSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QuarksStatefulSetSpec   `json:"spec,omitempty"`
	Status QuarksStatefulSetStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuarksStatefulSetList contains a list of QuarksStatefulSet
type QuarksStatefulSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QuarksStatefulSet `json:"items"`
}

// GetMaxAvailableVersion gets the greatest available version owned by the QuarksStatefulSet
func (q *QuarksStatefulSet) GetMaxAvailableVersion(versions map[int]bool) int {
	maxAvailableVersion := 0

	for version, available := range versions {
		if available && version > maxAvailableVersion {
			maxAvailableVersion = version
		}
	}
	return maxAvailableVersion
}
