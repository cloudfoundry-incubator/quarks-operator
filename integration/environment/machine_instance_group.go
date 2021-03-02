package environment

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

// WaitForInstanceGroup blocks until all selected pods of the instance group are running. It fails after the timeout.
func (m *Machine) WaitForInstanceGroup(namespace string, deployment string, igName string, count int) error {
	labels := labels.Set(map[string]string{
		bdv1.LabelDeploymentName:    deployment,
		bdv1.LabelInstanceGroupName: igName,
	}).String()
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		ss, err := m.GetStatefulSetByInstanceGroupName(namespace, igName)
		if err != nil {
			return false, nil
		}

		n, err := m.PodCount(namespace, labels, func(pod corev1.Pod) bool {
			return pod.Status.Phase == corev1.PodRunning && pod.Labels[appsv1.StatefulSetRevisionLabel] == ss.Status.UpdateRevision
		})
		if err != nil {
			return false, err
		}
		return n == count, nil
	})
}

// WaitForInstanceGroupVersions blocks until the specified number of pods from
// the instance group version are running. It fails after the timeout.
func (m *Machine) WaitForInstanceGroupVersions(namespace string, deployment string, igName string, count int, versions ...string) error {
	labels := labels.Set(map[string]string{
		bdv1.LabelDeploymentName:    deployment,
		bdv1.LabelInstanceGroupName: igName,
	}).String()
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {

		ss, err := m.GetStatefulSetByInstanceGroup(namespace, igName, versions)
		if err != nil {
			return false, nil
		}

		n, err := m.PodCount(namespace, labels, func(pod corev1.Pod) bool {
			return pod.Status.Phase == corev1.PodRunning &&
				pod.Labels[appsv1.StatefulSetRevisionLabel] == ss.Status.UpdateRevision
		})
		if err != nil {
			return false, err
		}
		return n == count, nil
	})
}

// GetInstanceGroupPods returns all pods from a specific instance group version
func (m *Machine) GetInstanceGroupPods(namespace string, deployment string, igName string) (*corev1.PodList, error) {
	labels := labels.Set(map[string]string{
		bdv1.LabelDeploymentName:    deployment,
		bdv1.LabelInstanceGroupName: igName,
	}).String()
	return m.GetPods(namespace, labels)
}
