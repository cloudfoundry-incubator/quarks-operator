package extendedjob

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ Query = &QueryImpl{}

// Query for events involving pods and filter them
type Query interface {
	Match(ejv1.ExtendedJob, corev1.Pod) bool
	MatchState(ejv1.ExtendedJob, ejv1.PodState) bool
}

// NewQuery returns a new Query struct
func NewQuery(c client.Client) *QueryImpl {
	return &QueryImpl{client: c}
}

// QueryImpl implements the query interface
type QueryImpl struct {
	client client.Client
}

// Match pod against label whitelist from extended job
func (q *QueryImpl) Match(extJob ejv1.ExtendedJob, pod corev1.Pod) bool {
	if pod.Name == "" {
		return false
	}

	// TODO https://github.com/kubernetes/apimachinery/blob/master/pkg/labels/selector.go
	match := extJob.Spec.Triggers.Selector.MatchLabels

	if labels.AreLabelsInWhiteList(match, pod.Labels) {
		return true
	}
	return false
}

// MatchState checks pod state against state from extended job
func (q *QueryImpl) MatchState(extJob ejv1.ExtendedJob, podState ejv1.PodState) bool {
	if extJob.Spec.Triggers.When == podState {
		return true
	}
	return false
}
