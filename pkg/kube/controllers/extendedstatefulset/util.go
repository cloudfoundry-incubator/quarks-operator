package extendedstatefulset

import (
	"context"

	"k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// listStatefulSets gets all StatefulSets cross version owned by the ExtendedStatefulSet
func listStatefulSets(ctx context.Context, client crc.Client, exStatefulSet *estsv1.ExtendedStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	secretLabelsSet := labels.Set{
		bdm.LabelDeploymentName:    exStatefulSet.Spec.Template.Labels[bdm.LabelDeploymentName],
		bdm.LabelInstanceGroupName: exStatefulSet.Spec.Template.Labels[bdm.LabelInstanceGroupName],
	}

	result := []v1beta2.StatefulSet{}

	// Get owned resources
	// Go through each StatefulSet
	allStatefulSets := &v1beta2.StatefulSetList{}
	err := client.List(
		ctx,
		&crc.ListOptions{
			Namespace:     exStatefulSet.Namespace,
			LabelSelector: secretLabelsSet.AsSelector(),
		},
		allStatefulSets)
	if err != nil {
		return nil, err
	}

	for _, statefulSet := range allStatefulSets.Items {
		if metav1.IsControlledBy(&statefulSet, exStatefulSet) {
			result = append(result, statefulSet)
			ctxlog.Debug(ctx, "StatefulSet '", statefulSet.Name, "' owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")
		} else {
			ctxlog.Debug(ctx, "StatefulSet '", statefulSet.Name, "' is not owned by ExtendedStatefulSet '", exStatefulSet.Name, "', ignoring.")
		}
	}

	return result, nil
}
