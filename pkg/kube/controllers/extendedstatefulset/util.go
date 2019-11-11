package extendedstatefulset

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1beta2client "k8s.io/client-go/kubernetes/typed/apps/v1beta2"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	estsv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

// GetMaxStatefulSetVersion returns the max version statefuleset
// of the extendedstatefulset.
func GetMaxStatefulSetVersion(ctx context.Context, client crc.Client, exStatefulSet *estsv1.ExtendedStatefulSet) (*v1beta2.StatefulSet, int, error) {
	// Default response is an empty StatefulSet with version '0' and an empty signature
	result := &v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				estsv1.AnnotationVersion: "0",
			},
		},
	}
	maxVersion := 0

	statefulSets, err := listStatefulSetsFromInformer(ctx, client, exStatefulSet)
	if err != nil {
		return nil, 0, err
	}

	ctxlog.Debug(ctx, "Getting the latest StatefulSet owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	for _, ss := range statefulSets {
		strVersion := ss.Annotations[estsv1.AnnotationVersion]
		if strVersion == "" {
			return nil, 0, errors.Errorf("The statefulset %s does not have the annotation(%s), a version could not be retrieved.", ss.Name, estsv1.AnnotationVersion)
		}

		version, err := strconv.Atoi(strVersion)
		if err != nil {
			return nil, 0, err
		}

		if ss.Annotations != nil && version > maxVersion {
			result = &ss
			maxVersion = version
		}
	}

	return result, maxVersion, nil
}

// listStatefulSetsFromInformer gets StatefulSets cross version owned by the ExtendedStatefulSet from informer
func listStatefulSetsFromInformer(ctx context.Context, client crc.Client, exStatefulSet *estsv1.ExtendedStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

	allStatefulSets := &v1beta2.StatefulSetList{}
	err := client.List(ctx, allStatefulSets,
		crc.InNamespace(exStatefulSet.Namespace),
	)
	if err != nil {
		return nil, err
	}

	return getStatefulSetsOwnedBy(ctx, exStatefulSet, allStatefulSets.Items)
}

// listStatefulSetsFromAPIClient gets StatefulSets cross version owned by the ExtendedStatefulSet from API client directly
func listStatefulSetsFromAPIClient(ctx context.Context, client appsv1beta2client.AppsV1beta2Interface, exStatefulSet *estsv1.ExtendedStatefulSet) ([]v1beta2.StatefulSet, error) {
	ctxlog.Debug(ctx, "Listing StatefulSets owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")

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
			ctxlog.Debug(ctx, "Found StatefulSet '", statefulSet.Name, "' owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")
		}
	}

	if len(result) == 0 {
		ctxlog.Debug(ctx, "Did not find any StatefulSet owned by ExtendedStatefulSet '", exStatefulSet.Name, "'.")
	}

	return result, nil
}
