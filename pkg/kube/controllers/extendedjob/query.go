package extendedjob

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ Query = &QueryImpl{}

// Query for events involving pods and filter them
type Query interface {
	Match(v1alpha1.ExtendedJob, corev1.Pod) bool
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
func (q *QueryImpl) Match(job v1alpha1.ExtendedJob, pod corev1.Pod) bool {
	// TODO pod state

	// TODO https://github.com/kubernetes/apimachinery/blob/master/pkg/labels/selector.go
	match := job.Spec.Triggers.Selector.MatchLabels

	if labels.AreLabelsInWhiteList(match, pod.Labels) {
		return true
	}
	return false
}
