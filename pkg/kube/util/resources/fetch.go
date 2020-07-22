package resources

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	crc "sigs.k8s.io/controller-runtime/pkg/client"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	qstsv1a1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
)

// ListBOSHDeployments fetches all the boshdeployments from the namespace
func ListBOSHDeployments(ctx context.Context, client crc.Client, namespace string) (*bdv1.BOSHDeploymentList, error) {
	result := &bdv1.BOSHDeploymentList{}
	err := client.List(ctx, result, crc.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list BOSHDeployments")
	}

	return result, nil
}

// ListQuarksStatefulSets fetches all the quarksstatefulsets from the namespace
func ListQuarksStatefulSets(ctx context.Context, client crc.Client, namespace string) (*qstsv1a1.QuarksStatefulSetList, error) {
	result := &qstsv1a1.QuarksStatefulSetList{}
	err := client.List(ctx, result, crc.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list QuarksStatefulSets")
	}

	return result, nil
}

// ListPods fetches all the pods from the namespace
func ListPods(ctx context.Context, client crc.Client, namespace string) (*corev1.PodList, error) {
	result := &corev1.PodList{}
	err := client.List(ctx, result, crc.InNamespace(namespace))
	if err != nil {
		return nil, errors.Wrap(err, "failed to list Pods")
	}

	return result, nil
}
