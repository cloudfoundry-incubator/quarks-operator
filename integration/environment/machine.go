package environment

import (
	"fmt"
	"strings"
	"time"

	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Machine produces and destroys resources for tests
type Machine struct {
	pollTimeout  time.Duration
	pollInterval time.Duration

	Clientset          *kubernetes.Clientset
	VersionedClientset *versioned.Clientset
}

// TearDownFunc tears down the resource
type TearDownFunc func()

func (m *Machine) CreateDefaultPod(namespace string, pod corev1.Pod) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Pods(namespace)
	_, err := client.Create(&pod)
	return func() {
		client.Delete(pod.GetName(), &metav1.DeleteOptions{})
	}, err
}

// WaitForPod blocks until the pod is running. It fails after the timeout.
func (m *Machine) WaitForPod(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PodRunning(namespace, name)
	})
}

// WaitForPods blocks until all selected pods are running. It fails after the timeout.
func (m *Machine) WaitForPods(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PodsRunning(namespace, labels)
	})
}

// WaitForExtendedStatefulSets blocks until at least one WaitForExtendedStatefulSet is found. It fails after the timeout.
func (m *Machine) WaitForExtendedStatefulSets(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.ExtendedStatefulSetExists(namespace, labels)
	})
}

// WaitForExtendedStatefulSetAvailable blocks until latest version is available. It fails after the timeout.
func (m *Machine) WaitForExtendedStatefulSetAvailable(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.ExtendedStatefulSetAvailable(namespace, name)
	})
}

// ExtendedStatefulSetExists returns true if at least one ess selected by labels exists
func (m *Machine) ExtendedStatefulSetExists(namespace string, labels string) (bool, error) {
	esss, err := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace).List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for ess by labels: %v", labels)
	}

	return len(esss.Items) > 0, nil
}

// ExtendedStatefulSetAvailable returns true if at least one latest version pod is running
func (m *Machine) ExtendedStatefulSetAvailable(namespace string, name string) (bool, error) {
	fieldSelector := fields.Set{"metadata.name": name}.AsSelector()
	esss, err := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace).List(metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for ess by name: %v", name)
	}

	if len(esss.Items) != 1 {
		return false, nil
	}

	ess := esss.Items[0]

	if len(ess.Status.Versions) == 0 {
		return false, nil
	}

	var latestVersion int
	for latestVersion = range ess.Status.Versions {
		break
	}
	for n := range ess.Status.Versions {
		if n > latestVersion {
			latestVersion = n
		}
	}

	if ess.Status.Versions[latestVersion] {
		return true, nil
	}

	return false, nil
}

// WaitForPodsDelete blocks until the pod is deleted. It fails after the timeout.
func (m *Machine) WaitForPodsDelete(namespace string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PodsDeleted(namespace)
	})
}

// PodsDeleted returns true if the all pods are deleted
func (m *Machine) PodsDeleted(namespace string) (bool, error) {
	podList, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	if len(podList.Items) == 0 {
		return true, nil
	}
	return false, nil
}

// PodRunning returns true if the pod by that name is in state running
func (m *Machine) PodRunning(namespace string, name string) (bool, error) {
	pod, err := m.Clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for pod by name: %s", name)
	}

	if pod.Status.Phase == corev1.PodRunning {
		return true, nil
	}
	return false, nil
}

// PodsRunning returns true if all the pods selected by labels are in state running
// Note that only the first page of pods is considered - don't use this if you have a
// long pod list that you care about
func (m *Machine) PodsRunning(namespace string, labels string) (bool, error) {
	pods, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for pod by labels: %v", labels)
	}

	if len(pods.Items) == 0 {
		return false, nil
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return false, nil
		}
	}

	return true, nil
}

// WaitForBOSHDeploymentDeletion blocks until the CR is deleted
func (m *Machine) WaitForBOSHDeploymentDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.HasBOSHDeployment(namespace, name)
		return !found, err
	})
}

// HasBOSHDeployment returns true if the pod by that name is in state running
func (m *Machine) HasBOSHDeployment(namespace string, name string) (bool, error) {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	_, err := client.Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for pod by name: %s", name)
	}

	return true, nil
}

// CreateConfigMap creates a ConfigMap and returns a function to delete it
func (m *Machine) CreateConfigMap(namespace string, configMap corev1.ConfigMap) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().ConfigMaps(namespace)
	_, err := client.Create(&configMap)
	return func() {
		client.Delete(configMap.GetName(), &metav1.DeleteOptions{})
	}, err
}

// CreateSecret creates a secret and returns a function to delete it
func (m *Machine) CreateSecret(namespace string, secret corev1.Secret) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	_, err := client.Create(&secret)
	return func() {
		client.Delete(secret.GetName(), &metav1.DeleteOptions{})
	}, err
}

// CreateBOSHDeployment creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) CreateBOSHDeployment(namespace string, deployment bdcv1.BOSHDeployment) (*bdcv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	d, err := client.Create(&deployment)
	return d, func() {
		client.Delete(deployment.GetName(), &metav1.DeleteOptions{})
	}, err
}

// UpdateBOSHDeployment creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) UpdateBOSHDeployment(namespace string, deployment bdcv1.BOSHDeployment) (*bdcv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	d, err := client.Update(&deployment)
	return d, func() {
		client.Delete(deployment.GetName(), &metav1.DeleteOptions{})
	}, err
}

// DeleteBOSHDeployment deletes a BOSHDeployment custom resource
func (m *Machine) DeleteBOSHDeployment(namespace string, name string) error {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// CreateExtendedStatefulSet creates a ExtendedStatefulSet custom resource and returns a function to delete it
func (m *Machine) CreateExtendedStatefulSet(namespace string, ess essv1.ExtendedStatefulSet) (*essv1.ExtendedStatefulSet, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Create(&ess)
	return d, func() {
		client.Delete(ess.GetName(), &metav1.DeleteOptions{})
	}, err
}

// GetExtendedStatefulSet gets a ExtendedStatefulSet custom resource
func (m *Machine) GetExtendedStatefulSet(namespace string, name string) (*essv1.ExtendedStatefulSet, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// UpdateExtendedStatefulSet creates a ExtendedStatefulSet custom resource and returns a function to delete it
func (m *Machine) UpdateExtendedStatefulSet(namespace string, ess essv1.ExtendedStatefulSet) (*essv1.ExtendedStatefulSet, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Update(&ess)
	return d, func() {
		client.Delete(ess.GetName(), &metav1.DeleteOptions{})
	}, err
}

// DeleteExtendedStatefulSet deletes a ExtendedStatefulSet custom resource
func (m *Machine) DeleteExtendedStatefulSet(namespace string, name string) error {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// PodLabeled returns true if the pod is labeled correctly
func (m *Machine) PodLabeled(namespace string, name string, desiredLabel, desiredValue string) (bool, error) {
	pod, err := m.Clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, err
		}
		return false, errors.Wrapf(err, "Failed to query for pod by name: %s", name)
	}

	if pod.ObjectMeta.Labels[desiredLabel] == desiredValue {
		return true, nil
	}
	return false, fmt.Errorf("Cannot match the desired label with %s", desiredValue)
}

// ContainExpectedEvent return true if events contain target resource event
func (m *Machine) ContainExpectedEvent(events []corev1.Event, reason string, message string) bool {
	for _, event := range events {
		if event.Reason == reason && strings.Contains(event.Message, message) {
			return true
		}
	}

	return false
}

// GetBOSHDeploymentEvents gets target resource events
func (m *Machine) GetBOSHDeploymentEvents(namespace string, name string, id string) ([]corev1.Event, error) {
	fieldSelector := fields.Set{"involvedObject.name": name, "involvedObject.uid": id}.AsSelector().String()
	err := m.WaitForBOSHDeploymentEvent(namespace, fieldSelector)
	if err != nil {
		return []corev1.Event{}, err
	}

	events := m.Clientset.CoreV1().Events(namespace)

	list, err := events.List(metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return []corev1.Event{}, err
	}
	return list.Items, nil
}

// WaitForBOSHDeploymentEvent gets desired event
func (m *Machine) WaitForBOSHDeploymentEvent(namespace string, fieldSelector string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.HasBOSHDeploymentEvent(namespace, fieldSelector)
		return found, err
	})
}

// HasBOSHDeploymentEvent returns true if the pod by that name is in state running
func (m *Machine) HasBOSHDeploymentEvent(namespace string, fieldSelector string) (bool, error) {
	events := m.Clientset.CoreV1().Events(namespace)
	eventList, err := events.List(metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		return false, err
	}
	if len(eventList.Items) == 0 {
		return false, nil
	}
	return true, nil
}
