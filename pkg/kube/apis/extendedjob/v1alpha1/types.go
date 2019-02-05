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
	Run                  Run                    `json:"run,omitempty"`
	Triggers             Triggers               `json:"triggers,omitempty"`
	Template             corev1.PodTemplateSpec `json:"template"`
	UpdateOnConfigChange bool                   `json:"updateOnConfigChange,omitempty"`
}

// Run is used if the job is not triggered
type Run string

const (
	// RunManually is the default for errand jobs
	RunManually Run = "manually"
	// RunNow instructs the controller to run the job now
	RunNow Run = "now"
	// RunOnce jobs run only once, when created
	RunOnce Run = "once"
)

// Output contains options to persist job output
type Output struct {
	SecretRef      string `json:"secretRef"`
	ConfigMapRef   string `json:"configMapRef"`
	WriteOnFailure bool   `json:"writeOnFailure"`
	OutputType     string `json:"outputType"`
}

// PodState is our abstraction of the pods state with regards to triggered
// extended jobs
type PodState string

const (
	// PodStateUnknown means we could not identify the state
	PodStateUnknown PodState = ""

	// PodStateReady means the pod is in phase=running with condition=ready
	PodStateReady PodState = "ready"

	// PodStateCreated means the pod did not exist before and is ready
	PodStateCreated PodState = "created"

	// PodStateNotReady means the pod is in phase pending
	PodStateNotReady PodState = "notready"

	// PodStateDeleted means the pod is in phase=succeeded or disappeared or phase=''
	PodStateDeleted PodState = "deleted"
)

// Triggers decide which objects to act on
type Triggers struct {
	When     PodState `json:"when"`
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
