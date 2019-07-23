package extendedstatefulset

import (
	"context"

	"k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	appsv1beta2client "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// listStatefulSetsFromInformer gets StatefulSets cross version owned by the ExtendedStatefulSet from informer
func listStatefulSetsFromInformer(ctx context.Context, client crc.Client, exStatefulSet *estsv1.ExtendedStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	allStatefulSets := &v1beta2.StatefulSetList{}
	err := client.List(
		ctx,
		&crc.ListOptions{
			Namespace:     exStatefulSet.Namespace,
			LabelSelector: labels.Everything(),
		},
		allStatefulSets)
	if err != nil {
		return nil, err
	}

	return getStatefulSetsOwnedBy(ctx, exStatefulSet, allStatefulSets.Items)
}

// listStatefulSetsFromAPIClient gets StatefulSets cross version owned by the ExtendedStatefulSet from API client directly
func listStatefulSetsFromAPIClient(ctx context.Context, client appsv1beta2client.AppsV1beta2Interface, exStatefulSet *estsv1.ExtendedStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	allStatefulSets := &v1beta2.StatefulSetList{}
	allStatefulSets, err := client.StatefulSets(exStatefulSet.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return getStatefulSetsOwnedBy(ctx, exStatefulSet, allStatefulSets.Items)
}

// getStatefulSetsOwnedBy gets StatefulSets owned by the ExtendedStatefulSet
func getStatefulSetsOwnedBy(ctx context.Context, exStatefulSet *estsv1.ExtendedStatefulSet, statefulSets []v1beta2.StatefulSet) ([]v1beta2.StatefulSet, error) {
	result := []v1beta2.StatefulSet{}

	for _, statefulSet := range statefulSets {
		if metav1.IsControlledBy(&statefulSet, exStatefulSet) {
			result = append(result, statefulSet)
			ctxlog.Debug(ctx, "StatefulSet '", statefulSet.Name, "' owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")
		} else {
			ctxlog.Debug(ctx, "StatefulSet '", statefulSet.Name, "' is not owned by ExtendedStatefulSet '", exStatefulSet.Name, "', ignoring.")
		}
	}

	return result, nil
}
