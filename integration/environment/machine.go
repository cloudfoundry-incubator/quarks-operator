package environment

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
	podutil "code.cloudfoundry.org/cf-operator/pkg/kube/util/pod"
)

// Machine produces and destroys resources for tests
type Machine struct {
	pollTimeout  time.Duration
	pollInterval time.Duration

	Clientset          *kubernetes.Clientset
	VersionedClientset *versioned.Clientset
}

// TearDownFunc tears down the resource
type TearDownFunc func() error

// CreateNamespace creates a namespace, it doesn't return an error if the namespace exists
func (m *Machine) CreateNamespace(namespace string) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Namespaces()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := client.Create(ns)
	if apierrors.IsAlreadyExists(err) {
		err = nil
	}
	return func() error {
		b := metav1.DeletePropagationBackground
		err := client.Delete(ns.GetName(), &metav1.DeleteOptions{
			// this is run in aftersuite before failhandler, so let's keep the namespace for a few seconds
			GracePeriodSeconds: util.Int64(5),
			PropagationPolicy:  &b,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// CreatePod creates a default pod and returns a function to delete it
func (m *Machine) CreatePod(namespace string, pod corev1.Pod) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Pods(namespace)
	_, err := client.Create(&pod)
	return func() error {
		err := client.Delete(pod.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
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

// WaitForPodFailures blocks until all selected pods are failing. It fails after the timeout.
func (m *Machine) WaitForPodFailures(namespace string, labels string) error {
	return wait.PollImmediate(5*time.Second, m.pollTimeout, func() (bool, error) {
		return m.PodsFailing(namespace, labels)
	})
}

// WaitForStatefulSetDelete blocks until the specified statefulset is deleted
func (m *Machine) WaitForStatefulSetDelete(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.StatefulSetExist(namespace, name)
		return !found, err
	})
}

// StatefulSetExist checks if the statefulset exists
func (m *Machine) StatefulSetExist(namespace string, name string) (bool, error) {
	_, err := m.Clientset.AppsV1beta1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for statefulset by name: %s", name)
	}
	return true, nil
}

// WaitForStatefulSetNewGeneration blocks until at least one StatefulSet is found. It fails after the timeout.
func (m *Machine) WaitForStatefulSetNewGeneration(namespace string, name string, currentVersion int64) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.StatefulSetNewGeneration(namespace, name, currentVersion)
	})
}

// WaitForExtendedStatefulSets blocks until at least one ExtendedStatefulSet is found. It fails after the timeout.
func (m *Machine) WaitForExtendedStatefulSets(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.ExtendedStatefulSetExists(namespace, labels)
	})
}

// WaitForExtendedStatefulSetAvailable blocks until latest version is available. It fails after the timeout.
func (m *Machine) WaitForExtendedStatefulSetAvailable(namespace string, name string, version int) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.ExtendedStatefulSetAvailable(namespace, name, version)
	})
}

// WaitForPV blocks until the pv is running. It fails after the timeout.
func (m *Machine) WaitForPV(name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PVAvailable(name)
	})
}

// PVAvailable returns true if the pv by that name is in state available
func (m *Machine) PVAvailable(name string) (bool, error) {
	pv, err := m.Clientset.CoreV1().PersistentVolumes().Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for pv by name: %s", name)
	}

	if pv.Status.Phase == "Available" {
		return true, nil
	}
	return false, nil
}

// WaitForPVsDelete blocks until the pv is deleted. It fails after the timeout.
func (m *Machine) WaitForPVsDelete(labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PVsDeleted(labels)
	})
}

// PVsDeleted returns true if the all pvs are deleted
func (m *Machine) PVsDeleted(labels string) (bool, error) {
	pvList, err := m.Clientset.CoreV1().PersistentVolumes().List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return false, err
	}
	if len(pvList.Items) == 0 {
		return true, nil
	}
	return false, nil
}

// WaitForPVCsDelete blocks until the pvc is deleted. It fails after the timeout.
func (m *Machine) WaitForPVCsDelete(namespace string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PVCsDeleted(namespace)
	})
}

// PVCsDeleted returns true if the all pvs are deleted
func (m *Machine) PVCsDeleted(namespace string) (bool, error) {
	pvcList, err := m.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	if len(pvcList.Items) == 0 {
		return true, nil
	}
	return false, nil
}

// WaitForStatefulSet blocks until all statefulset pods are running. It fails after the timeout.
func (m *Machine) WaitForStatefulSet(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.StatefulSetRunning(namespace, labels)
	})
}

// StatefulSetRunning returns true if the statefulset by that name has all pods created
func (m *Machine) StatefulSetRunning(namespace string, name string) (bool, error) {
	statefulSet, err := m.Clientset.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for statefulset by name: %s", name)
	}

	if statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas {
		return true, nil
	}
	return false, nil
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

// StatefulSetNewGeneration returns true if StatefulSet has new generation
func (m *Machine) StatefulSetNewGeneration(namespace string, name string, version int64) (bool, error) {
	client := m.Clientset.AppsV1beta1().StatefulSets(namespace)

	ss, err := client.Get(name, metav1.GetOptions{})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for statefulSet by name: %v", name)
	}

	if *ss.Status.ObservedGeneration > version {
		return true, nil
	}

	return false, nil
}

// ExtendedStatefulSetAvailable returns true if current version is available
func (m *Machine) ExtendedStatefulSetAvailable(namespace string, name string, version int) (bool, error) {
	latestVersion := version

	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)

	ess, err := client.Get(name, metav1.GetOptions{})
	if err != nil {
		return false, errors.Wrapf(err, "failed to query for extendedStatefulSet by name: %v", name)
	}

	if len(ess.Status.Versions) == 0 {
		return false, nil
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

// PodsFailing returns true if the pod by that name exist and is in a failed state
func (m *Machine) PodsFailing(namespace string, labels string) (bool, error) {
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

		pos, condition := podutil.GetPodCondition(&pod.Status, corev1.ContainersReady)
		if (pos > -1 && condition.Reason == "ContainersNotReady") ||
			pod.Status.Phase == corev1.PodFailed {

			return true, nil
		}
		for _, containerStatus := range pod.Status.ContainerStatuses {
			state := containerStatus.State
			if (state.Waiting != nil && state.Waiting.Reason == "ImagePullBackOff") ||
				(state.Waiting != nil && state.Waiting.Reason == "ErrImagePull") {
				return true, nil
			}
		}
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

// PodCount returns the number of matching pods
func (m *Machine) PodCount(namespace string, labels string) (int, error) {
	pods, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to query for pod by labels: %v", labels)
	}

	return len(pods.Items), nil
}

// GetPods returns all the pods selected by labels
func (m *Machine) GetPods(namespace string, labels string) (*corev1.PodList, error) {
	pods, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: labels,
	})
	if err != nil {
		return &corev1.PodList{}, errors.Wrapf(err, "failed to query for pod by labels: %v", labels)
	}

	return pods, nil

}

// GetPod returns pod by name
func (m *Machine) GetPod(namespace string, name string) (*corev1.Pod, error) {
	pod, err := m.Clientset.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query for pod by name: %v", name)
	}

	return pod, nil
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
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
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
	return func() error {
		err := client.Delete(configMap.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// CreateSecret creates a secret and returns a function to delete it
func (m *Machine) CreateSecret(namespace string, secret corev1.Secret) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	_, err := client.Create(&secret)
	return func() error {
		err := client.Delete(secret.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// UpdateConfigMap updates a ConfigMap and returns a function to delete it
func (m *Machine) UpdateConfigMap(namespace string, configMap corev1.ConfigMap) (*corev1.ConfigMap, TearDownFunc, error) {
	client := m.Clientset.CoreV1().ConfigMaps(namespace)
	cm, err := client.Update(&configMap)
	return cm, func() error {
		err := client.Delete(configMap.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// UpdateSecret updates a secret and returns a function to delete it
func (m *Machine) UpdateSecret(namespace string, secret corev1.Secret) (*corev1.Secret, TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	s, err := client.Update(&secret)
	return s, func() error {
		err := client.Delete(secret.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// CollectSecret polls untile the specified secret can be fetched
func (m *Machine) CollectSecret(namespace string, name string) (*corev1.Secret, error) {
	err := m.WaitForSecret(namespace, name)
	if err != nil {
		return nil, errors.Wrap(err, "waiting for secret "+name)
	}
	return m.GetSecret(namespace, name)
}

// GetSecret fetches the specified secret
func (m *Machine) GetSecret(namespace string, name string) (*corev1.Secret, error) {
	secret, err := m.Clientset.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "waiting for secret "+name)
	}

	return secret, nil
}

// WaitForSecret blocks until the secret is available. It fails after the timeout.
func (m *Machine) WaitForSecret(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.SecretExists(namespace, name)
	})
}

// WaitForSecretDeletion blocks until the CR is deleted
func (m *Machine) WaitForSecretDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.SecretExists(namespace, name)
		return !found, err
	})
}

// SecretExists returns true if the secret by that name exist
func (m *Machine) SecretExists(namespace string, name string) (bool, error) {
	_, err := m.Clientset.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for secret by name: %s", name)
	}

	return true, nil
}

// DeleteSecrets deletes all the secrets
func (m *Machine) DeleteSecrets(namespace string) (bool, error) {
	err := m.Clientset.CoreV1().Secrets(namespace).DeleteCollection(
		&metav1.DeleteOptions{},
		metav1.ListOptions{},
	)
	if err != nil {
		return false, errors.Wrapf(err, "failed to delete all secrets in namespace: %s", namespace)
	}

	return true, nil
}

// CreateBOSHDeployment creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) CreateBOSHDeployment(namespace string, deployment bdv1.BOSHDeployment) (*bdv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	d, err := client.Create(&deployment)
	return d, func() error {
		err := client.Delete(deployment.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// GetBOSHDeployment gets a BOSHDeployment custom resource
func (m *Machine) GetBOSHDeployment(namespace string, name string) (*bdv1.BOSHDeployment, error) {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// UpdateBOSHDeployment creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) UpdateBOSHDeployment(namespace string, deployment bdv1.BOSHDeployment) (*bdv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	d, err := client.Update(&deployment)
	return d, func() error {
		err := client.Delete(deployment.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// DeleteBOSHDeployment deletes a BOSHDeployment custom resource
func (m *Machine) DeleteBOSHDeployment(namespace string, name string) error {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// CreateExtendedSecret creates a ExtendedSecret custom resource and returns a function to delete it
func (m *Machine) CreateExtendedSecret(namespace string, es esv1.ExtendedSecret) (*esv1.ExtendedSecret, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedsecretV1alpha1().ExtendedSecrets(namespace)
	d, err := client.Create(&es)
	return d, func() error {
		err := client.Delete(es.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// DeleteExtendedSecret deletes an ExtendedSecret custom resource
func (m *Machine) DeleteExtendedSecret(namespace string, name string) error {
	client := m.VersionedClientset.ExtendedsecretV1alpha1().ExtendedSecrets(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// CreateExtendedStatefulSet creates a ExtendedStatefulSet custom resource and returns a function to delete it
func (m *Machine) CreateExtendedStatefulSet(namespace string, ess essv1.ExtendedStatefulSet) (*essv1.ExtendedStatefulSet, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)

	d, err := client.Create(&ess)

	return d, func() error {
		pvcs, err := m.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{
				"test-run-reference": ess.Name,
			}).String(),
		})
		if err != nil {
			return err
		}

		err = client.Delete(ess.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		for _, pvc := range pvcs.Items {
			err = m.Clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(pvc.GetName(), &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}

		return nil
	}, err
}

// GetExtendedStatefulSet gets a ExtendedStatefulSet custom resource
func (m *Machine) GetExtendedStatefulSet(namespace string, name string) (*essv1.ExtendedStatefulSet, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// CheckExtendedStatefulSet checks extendedstatefulset according to the version
func (m *Machine) CheckExtendedStatefulSet(namespace string, name string, version int) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.CheckExtendedStatefulSetVersion(namespace, name, version)
	})
}

// CheckExtendedStatefulSetVersion returns true if the version status is true
func (m *Machine) CheckExtendedStatefulSetVersion(namespace string, name string, version int) (bool, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	if d.Status.Versions[version] && len(d.Status.Versions) == 1 {
		return true, nil
	}
	return false, err
}

// UpdateExtendedStatefulSet updates a ExtendedStatefulSet custom resource and returns a function to delete it
func (m *Machine) UpdateExtendedStatefulSet(namespace string, ess essv1.ExtendedStatefulSet) (*essv1.ExtendedStatefulSet, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedstatefulsetV1alpha1().ExtendedStatefulSets(namespace)
	d, err := client.Update(&ess)
	return d, func() error {
		err := client.Delete(ess.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
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
		return false, errors.Wrapf(err, "failed to query for pod by name: %s", name)
	}

	if pod.ObjectMeta.Labels[desiredLabel] == desiredValue {
		return true, nil
	}
	return false, fmt.Errorf("cannot match the desired label with %s", desiredValue)
}

// DeleteJobs deletes all the jobs
func (m *Machine) DeleteJobs(namespace string, labels string) (bool, error) {
	err := m.Clientset.BatchV1().Jobs(namespace).DeleteCollection(
		&metav1.DeleteOptions{},
		metav1.ListOptions{LabelSelector: labels},
	)
	if err != nil {
		return false, errors.Wrapf(err, "failed to delete all jobs with labels: %s", labels)
	}

	return true, nil
}

// WaitForJobsDeleted waits until the jobs no longer exists
func (m *Machine) WaitForJobsDeleted(namespace string, labels string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to list jobs by label: %s", labels)
		}

		return len(jobs.Items) < 1, nil
	})
}

// JobExists returns true if job with that name exists
func (m *Machine) JobExists(namespace string, name string) (bool, error) {
	_, err := m.Clientset.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for job by name: %s", name)
	}

	return true, nil
}

// CollectJobs waits for n jobs with specified labels.
// It fails after the timeout.
func (m *Machine) CollectJobs(namespace string, labels string, n int) ([]batchv1.Job, error) {
	found := map[string]batchv1.Job{}
	err := wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for jobs by label: %s", labels)
		}

		for _, job := range jobs.Items {
			found[job.GetName()] = job
		}
		return len(found) >= n, nil
	})

	if err != nil {
		return nil, err
	}

	jobs := []batchv1.Job{}
	for _, job := range found {
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// WaitForJobExists polls until a short timeout is reached or a job is found
// It returns true only if a job is found
func (m *Machine) WaitForJobExists(namespace string, labels string) (bool, error) {
	found := false
	err := wait.Poll(5*time.Second, 30*time.Second, func() (bool, error) {
		jobs, err := m.Clientset.BatchV1().Jobs(namespace).List(metav1.ListOptions{
			LabelSelector: labels,
		})
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for jobs by label: %s", labels)
		}

		found = len(jobs.Items) != 0
		return found, err
	})

	if err != nil && strings.Contains(err.Error(), "timed out waiting for the condition") {
		err = nil
	}

	return found, err
}

// WaitForJobDeletion blocks until the batchv1.Job is deleted
func (m *Machine) WaitForJobDeletion(namespace string, name string) error {
	return wait.PollImmediate(1*time.Second, 30*time.Second, func() (bool, error) {
		found, err := m.JobExists(namespace, name)
		return !found, err
	})
}

// ContainJob searches job array for a job matching `name`
func (m *Machine) ContainJob(jobs []batchv1.Job, name string) bool {
	for _, job := range jobs {
		if strings.Contains(job.GetName(), name) {
			return true
		}
	}
	return false
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

// GetStatefulSet gets a StatefulSet custom resource
func (m *Machine) GetStatefulSet(namespace string, name string) (*v1beta1.StatefulSet, error) {
	statefulSet, err := m.Clientset.AppsV1beta1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return &v1beta1.StatefulSet{}, errors.Wrapf(err, "failed to query for statefulSet by name: %v", name)
	}

	return statefulSet, nil
}

// GetConfigMap gets a ConfigMap by name
func (m *Machine) GetConfigMap(namespace string, name string) (*corev1.ConfigMap, error) {
	configMap, err := m.Clientset.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return &corev1.ConfigMap{}, errors.Wrapf(err, "failed to query for configMap by name: %v", name)
	}

	return configMap, nil
}

// GetExtendedJob gets an ExtendedJob custom resource
func (m *Machine) GetExtendedJob(namespace string, name string) (*ejv1.ExtendedJob, error) {
	client := m.VersionedClientset.ExtendedjobV1alpha1().ExtendedJobs(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// CreateExtendedJob creates an ExtendedJob
func (m *Machine) CreateExtendedJob(namespace string, job ejv1.ExtendedJob) (*ejv1.ExtendedJob, TearDownFunc, error) {
	client := m.VersionedClientset.ExtendedjobV1alpha1().ExtendedJobs(namespace)
	d, err := client.Create(&job)
	return d, func() error {
		pods, err := m.Clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{
			LabelSelector: labels.Set(map[string]string{
				ejv1.LabelEJobName: job.Name,
			}).String(),
		})
		if err != nil {
			return err
		}

		err = client.Delete(job.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}

		for _, pod := range pods.Items {
			err = m.Clientset.CoreV1().Pods(namespace).Delete(pod.GetName(), &metav1.DeleteOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}

		return nil
	}, err
}

// UpdateExtendedJob updates an extended job
func (m *Machine) UpdateExtendedJob(namespace string, eJob ejv1.ExtendedJob) error {
	client := m.VersionedClientset.ExtendedjobV1alpha1().ExtendedJobs(namespace)
	_, err := client.Update(&eJob)
	return err
}

// WaitForExtendedJobDeletion blocks until the CR job is deleted
func (m *Machine) WaitForExtendedJobDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.ExtendedJobExists(namespace, name)
		return !found, err
	})
}

// ExtendedJobExists returns true if extended job with that name exists
func (m *Machine) ExtendedJobExists(namespace string, name string) (bool, error) {
	_, err := m.GetExtendedJob(namespace, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for extended job by name: %s", name)
	}

	return true, nil
}

// TearDownAll calls all passed in tear down functions in order
func (m *Machine) TearDownAll(funcs []TearDownFunc) error {
	var messages string
	for _, f := range funcs {
		err := f()
		if err != nil {
			messages = fmt.Sprintf("%v%v\n", messages, err.Error())
		}
	}
	if messages != "" {
		return errors.New(messages)
	}
	return nil
}

// GetService gets target Service
func (m *Machine) GetService(namespace string, name string) (*corev1.Service, error) {
	svc, err := m.Clientset.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return svc, errors.Wrapf(err, "failed to get service '%s'", svc)
	}

	return svc, nil
}

// GetEndpoints gets target Endpoints
func (m *Machine) GetEndpoints(namespace string, name string) (*corev1.Endpoints, error) {
	ep, err := m.Clientset.CoreV1().Endpoints(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return ep, errors.Wrapf(err, "failed to get endpoint '%s'", ep)
	}

	return ep, nil
}

// WaitForSubsetsExist blocks until the specified endpoints' subsets exist
func (m *Machine) WaitForSubsetsExist(namespace string, endpointsName string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.SubsetsExist(namespace, endpointsName)
		return found, err
	})
}

// SubsetsExist checks if the subsets of the endpoints exist
func (m *Machine) SubsetsExist(namespace string, endpointsName string) (bool, error) {
	ep, err := m.Clientset.CoreV1().Endpoints(namespace).Get(endpointsName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for endpoints by endpointsName: %s", endpointsName)
	}

	if len(ep.Subsets) == 0 {
		return false, nil
	}

	return true, nil
}
