package quarksstatefulset

import (
	"context"

	"k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1beta2client "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// listStatefulSetsFromInformer gets StatefulSets cross version owned by the QuarksStatefulSet from informer
func listStatefulSetsFromInformer(ctx context.Context, client crc.Client, qStatefulSet *qstsv1a1.QuarksStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by QuarksStatefulSet '", qStatefulSet.Name, "'.")

	allStatefulSets := &v1beta2.StatefulSetList{}
	err := client.List(ctx, allStatefulSets,
		crc.InNamespace(qStatefulSet.Namespace),
	)
	if err != nil {
		return nil, err
	}

	return getStatefulSetsOwnedBy(ctx, qStatefulSet, allStatefulSets.Items)
}

// listStatefulSetsFromAPIClient gets StatefulSets cross version owned by the QuarksStatefulSet from API client directly
func listStatefulSetsFromAPIClient(ctx context.Context, client appsv1beta2client.AppsV1beta2Interface, qStatefulSet *qstsv1a1.QuarksStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by QuarksStatefulSet '", qStatefulSet.Name, "'.")

	allStatefulSets, err := client.StatefulSets(qStatefulSet.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return getStatefulSetsOwnedBy(ctx, qStatefulSet, allStatefulSets.Items)
}

// getStatefulSetsOwnedBy gets StatefulSets owned by the QuarksStatefulSet
func getStatefulSetsOwnedBy(ctx context.Context, qStatefulSet *qstsv1a1.QuarksStatefulSet, statefulSets []v1beta2.StatefulSet) ([]v1beta2.StatefulSet, error) {
	result := []v1beta2.StatefulSet{}

	for _, statefulSet := range statefulSets {
		if metav1.IsControlledBy(&statefulSet, qStatefulSet) {
			result = append(result, statefulSet)
			ctxlog.Debug(ctx, "Found StatefulSet '", statefulSet.Name, "' owned by QuarksStatefulSet '", qStatefulSet.Name, "'.")
		}
	}

	if len(result) == 0 {
		ctxlog.Debug(ctx, "Did not find any StatefulSet owned by QuarksStatefulSet '", qStatefulSet.Name, "'.")
	}

	return result, nil
}
