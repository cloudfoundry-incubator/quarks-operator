package extendedjob

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ Query = &QueryImpl{}

// Query for events involving pods and filter them
type Query interface {
	Match(ejv1.ExtendedJob, corev1.Pod) bool
	MatchState(ejv1.ExtendedJob, ejv1.PodState) bool
}

// NewQuery returns a new Query struct
func NewQuery() *QueryImpl {
	return &QueryImpl{}
}

// QueryImpl implements the query interface
type QueryImpl struct {
}

// Match pod against label whitelist from extended job
func (q *QueryImpl) Match(extJob ejv1.ExtendedJob, pod corev1.Pod) bool {
	if pod.Name == "" {
		return false
	}
	if extJob.Spec.Trigger.PodState.Selector == nil {
		return true
	}

	podLabelsSet := labels.Set(pod.Labels)
	matchExpressions := extJob.Spec.Trigger.PodState.Selector.MatchExpressions
	for _, exp := range matchExpressions {
		requirement, err := labels.NewRequirement(exp.Key, exp.Operator, exp.Values)
		if err != nil {
			// q.log.Errorf("Error converting requirement '%#v': %s", exp, err)
		} else if !requirement.Matches(podLabelsSet) {
			return false
		}
	}

	// TODO https://github.com/kubernetes/apimachinery/blob/master/pkg/labels/selector.go
	matchLabels := extJob.Spec.Trigger.PodState.Selector.MatchLabels
	if labels.AreLabelsInWhiteList(*matchLabels, pod.Labels) {
		return true
	}
	return false
}

// MatchState checks pod state against state from extended job
func (q *QueryImpl) MatchState(extJob ejv1.ExtendedJob, podState ejv1.PodState) bool {
	if extJob.Spec.Trigger.PodState != nil && extJob.Spec.Trigger.PodState.When == podState {
		return true
	}
	return false
}
