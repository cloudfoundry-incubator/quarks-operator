package extendedjob

import (
	"go.uber.org/zap"
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
func NewQuery(c client.Client, log *zap.SugaredLogger) *QueryImpl {
	return &QueryImpl{client: c, log: log}
}

// QueryImpl implements the query interfacepod.Labels
type QueryImpl struct {
	client client.Client
	log    *zap.SugaredLogger
}

// Match pod against label whitelist from extended job
func (q *QueryImpl) Match(extJob ejv1.ExtendedJob, pod corev1.Pod) bool {
	if pod.Name == "" {
		return false
	}

	podLabelsSet := labels.Set(pod.Labels)
	matchExpressions := extJob.Spec.Triggers.Selector.MatchExpressions
	for _, exp := range matchExpressions {
		requirement, err := labels.NewRequirement(exp.Key, exp.Operator, exp.Values)
		if err != nil {
			q.log.Errorf("Error converting requirement '%#v': %s", exp, err)
		} else if !requirement.Matches(podLabelsSet) {
			return false
		}
	}

	// TODO https://github.com/kubernetes/apimachinery/blob/master/pkg/labels/selector.go
	matchLabels := extJob.Spec.Triggers.Selector.MatchLabels
	if labels.AreLabelsInWhiteList(matchLabels, pod.Labels) {
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
