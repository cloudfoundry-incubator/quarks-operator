package environment

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
)

// WaitForInstanceGroup blocks until all selected pods of the instance group are running. It fails after the timeout.
func (m *Machine) WaitForInstanceGroup(namespace string, deployment string, igName string, version string, count int) error {
	labels := labels.Set(map[string]string{
		bdm.LabelDeploymentName:    deployment,
		bdm.LabelInstanceGroupName: igName,
		bdm.LabelDeploymentVersion: version,
	}).String()
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		n, err := m.PodCount(namespace, labels, func(pod corev1.Pod) bool {
			return pod.Status.Phase == corev1.PodRunning
		})
		if err != nil {
			return false, err
		}
		return n == count, nil
	})
}

// GetInstanceGroupPods returns all pods from a specific instance group version
func (m *Machine) GetInstanceGroupPods(namespace string, deployment string, igName string, version string) (*corev1.PodList, error) {
	labels := labels.Set(map[string]string{
		bdm.LabelDeploymentName:    deployment,
		bdm.LabelInstanceGroupName: igName,
		bdm.LabelDeploymentVersion: version,
	}).String()
	return m.GetPods(namespace, labels)
}
