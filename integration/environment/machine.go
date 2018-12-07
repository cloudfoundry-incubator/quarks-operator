package environment

import (
	"fmt"
	"time"

	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// WaitForPod blocks until the pod is running. It fails after the timeout.
func (m *Machine) WaitForPod(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		return m.PodRunning(namespace, name)
	})
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
	pod, err := m.Clientset.CoreV1().Pods(namespace).Get(name, v1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for pod by name: %s", name)
	}

	if pod.Status.Phase == apiv1.PodRunning {
		return true, nil
	}
	return false, nil
}

// PodLabeled returns true if the pod is interpolated correctly
func (m *Machine) PodLabeled(namespace string, name string, desiredLabel, desiredValue string) (bool, error) {
	pod, err := m.Clientset.CoreV1().Pods(namespace).Get(name, v1.GetOptions{})
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

// WaitForCRDeletion blocks until the CR is deleted
func (m *Machine) WaitForCRDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.pollInterval, m.pollTimeout, func() (bool, error) {
		found, err := m.HasFissileCR(namespace, name)
		return !found, err
	})
}

// HasFissileCR returns true if the pod by that name is in state running
func (m *Machine) HasFissileCR(namespace string, name string) (bool, error) {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	_, err := client.Get(name, v1.GetOptions{})
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
		client.Delete(configMap.GetName(), &v1.DeleteOptions{})
	}, err
}

// CreateSecret creates a secret and returns a function to delete it
func (m *Machine) CreateSecret(namespace string, secret corev1.Secret) (TearDownFunc, error) {
	client := m.Clientset.CoreV1().Secrets(namespace)
	_, err := client.Create(&secret)
	return func() {
		client.Delete(secret.GetName(), &v1.DeleteOptions{})
	}, err
}

// CreateFissileCR creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) CreateFissileCR(namespace string, deployment bdcv1.BOSHDeployment) (*bdcv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	d, err := client.Create(&deployment)
	return d, func() {
		client.Delete(deployment.GetName(), &v1.DeleteOptions{})
	}, err
}

// UpdateFissileCR creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) UpdateFissileCR(namespace string, deployment bdcv1.BOSHDeployment) (*bdcv1.BOSHDeployment, TearDownFunc, error) {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	d, err := client.Update(&deployment)
	return d, func() {
		client.Delete(deployment.GetName(), &v1.DeleteOptions{})
	}, err
}

// DeleteFissileCR deletes a BOSHDeployment custom resource
func (m *Machine) DeleteFissileCR(namespace string, name string) error {
	client := m.VersionedClientset.Boshdeployment().BOSHDeployments(namespace)
	return client.Delete(name, &v1.DeleteOptions{})
}
