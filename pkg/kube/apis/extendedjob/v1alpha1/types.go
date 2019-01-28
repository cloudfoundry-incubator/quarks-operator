package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// This file is safe to edit
// It's used as input for the Kube code generator
// Run "make generate" after modifying this file

// ExtendedJobSpec defines the desired state of ExtendedJob
type ExtendedJobSpec struct {
	Output               Output                 `json:"output,omitempty"`
	Run                  string                 `json:"run,omitempty"`
	Triggers             Triggers               `json:"triggers,omitempty"`
	Template             corev1.PodTemplateSpec `json:"template"`
	UpdateOnConfigChange bool                   `json:"updateOnConfigChange,omitempty"`
}

// Output contains options to persist job output
type Output struct {
	SecretRef      string `json:"secretRef"`
	ConfigMapRef   string `json:"configMapRef"`
	WriteOnFailure bool   `json:"writeOnFailure"`
	OutputType     string `json:"outputType"`
}

// Triggers decide which objects to act on
type Triggers struct {
	When     string   `json:"when"`
	Selector Selector `json:"selector,omitempty"`
}

// Selector filter objects
type Selector struct {
	MatchLabels      labels.Set           `json:"matchLabels,omitempty"`
	MatchExpressions []labels.Requirement `json:"matchExpressions,omitempty"`
}

// ExtendedJobStatus defines the observed state of ExtendedJob
type ExtendedJobStatus struct {
	Nodes []string `json:"nodes"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedJob is the Schema for the extendedstatefulsetcontroller API
// +k8s:openapi-gen=true
type ExtendedJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExtendedJobSpec   `json:"spec,omitempty"`
	Status ExtendedJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtendedJobList contains a list of ExtendedJob
type ExtendedJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ExtendedJob `json:"items"`
}
