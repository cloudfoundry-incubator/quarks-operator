package extendedjob

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

var _ Query = &QueryImpl{}

// Query for events involving pods and filter them
type Query interface {
	Match(ejv1.ExtendedJob, corev1.Pod) bool
	MatchState(ejv1.ExtendedJob, ejv1.PodState) bool
	IsJobAlreadyExists(context.Context, ejv1.ExtendedJob, types.UID, *TriggerReconciler) (bool, error)
}

// NewQuery returns a new Query struct
func NewQuery() *QueryImpl {
	return &QueryImpl{}
}

// QueryImpl implements the query interface
type QueryImpl struct {
}

// Match pod against label whitelist from extended job
func (q *QueryImpl) Match(eJob ejv1.ExtendedJob, pod corev1.Pod) bool {
	if pod.Name == "" {
		return false
	}
	if eJob.Spec.Trigger.PodState.Selector == nil {
		return true
	}

	podLabelsSet := labels.Set(pod.Labels)
	matchExpressions := eJob.Spec.Trigger.PodState.Selector.MatchExpressions
	for _, exp := range matchExpressions {
		requirement, err := labels.NewRequirement(exp.Key, exp.Operator, exp.Values)
		if err != nil {
			// q.log.Errorf("Error converting requirement '%#v': %s", exp, err)
		} else if !requirement.Matches(podLabelsSet) {
			return false
		}
	}

	// TODO https://github.com/kubernetes/apimachinery/blob/master/pkg/labels/selector.go
	matchLabels := eJob.Spec.Trigger.PodState.Selector.MatchLabels
	if labels.AreLabelsInWhiteList(*matchLabels, pod.Labels) {
		return true
	}
	return false
}

// MatchState checks pod state against state from extended job
func (q *QueryImpl) MatchState(eJob ejv1.ExtendedJob, podState ejv1.PodState) bool {
	if eJob.Spec.Trigger.PodState != nil && eJob.Spec.Trigger.PodState.When == podState {
		return true
	}
	return false
}

// IsJobAlreadyExists checks if the job is already created for the pod event match with exjob
func (q *QueryImpl) IsJobAlreadyExists(ctx context.Context, eJob ejv1.ExtendedJob, podUID types.UID, r *TriggerReconciler) (bool, error) {

	// Fetch all jobs using label
	podList := &corev1.PodList{}

	podLabels := labels.Set{
		"ejob-name": eJob.GetName(),
	}

	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     eJob.GetNamespace(),
		LabelSelector: podLabels.AsSelector(),
	}, podList)
	if err != nil {
		return false, err
	}

	for _, pod := range podList.Items {
		if strings.Contains(pod.GetName(), string(podUID)) {
			return true, nil
		}
	}
	return false, nil
}
