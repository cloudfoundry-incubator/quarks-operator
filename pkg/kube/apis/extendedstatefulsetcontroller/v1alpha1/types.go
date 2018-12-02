package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// ExtendedStatefulSetSpec defines the desired state of ExtendedStatefulSet
type ExtendedStatefulSetSpec struct {
}

// ExtendedStatefulSetStatus defines the observed state of ExtendedStatefulSet
type ExtendedStatefulSetStatus struct {
	Nodes []string `json:"nodes"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedStatefulSet is the Schema for the extendedstatefulsetcontroller API
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
