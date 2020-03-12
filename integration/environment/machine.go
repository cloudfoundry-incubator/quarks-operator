package environment

import (
	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"code.cloudfoundry.org/cf-operator/pkg/kube/client/clientset/versioned"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// Machine produces and destroys resources for tests
type Machine struct {
	machine.Machine

	VersionedClientset *versioned.Clientset
}

// CreateStatefulSet creates a statefulset and returns a function to delete it
func (m *Machine) CreateStatefulSet(namespace string, res appsv1.StatefulSet) (machine.TearDownFunc, error) {
	client := m.Clientset.AppsV1().StatefulSets(namespace)
	_, err := client.Create(&res)
	return func() error {
		err := client.Delete(res.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// CreateDeployment creates a statefulset and returns a function to delete it
func (m *Machine) CreateDeployment(namespace string, res appsv1.Deployment) (machine.TearDownFunc, error) {
	client := m.Clientset.AppsV1().Deployments(namespace)
	_, err := client.Create(&res)
	return func() error {
		err := client.Delete(res.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// CollectStatefulSet waits for statefulset with generation > x to appear, then returns it
func (m *Machine) CollectStatefulSet(namespace string, name string, generation int64) (*appsv1.StatefulSet, error) {
	err := m.WaitForStatefulSetNewGeneration(namespace, name, generation)
	if err != nil {
		return nil, errors.Wrap(err, "waiting for statefulset "+name)
	}
	return m.GetStatefulSet(namespace, name)
}

// WaitForDeployment waits for deployment with generation > x to appear
func (m *Machine) WaitForDeployment(namespace string, name string, generation int64) error {
	client := m.Clientset.AppsV1().Deployments(namespace)
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		d, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "failed to query for deployment by name: %v", name)
		}

		if d.Status.ReadyReplicas != *d.Spec.Replicas {
			return false, nil
		}

		if d.Status.ObservedGeneration > generation {
			return true, nil
		}

		return false, nil
	})
}

// CollectDeployment waits for deployment with generation > x to appear, then returns it
func (m *Machine) CollectDeployment(namespace string, name string, generation int64) (*appsv1.Deployment, error) {
	client := m.Clientset.AppsV1().Deployments(namespace)
	err := m.WaitForDeployment(namespace, name, generation)
	if err != nil {
		return nil, errors.Wrap(err, "waiting for deployment "+name)
	}
	return client.Get(name, metav1.GetOptions{})
}

// EnvKeys returns an array of all env key names found in containers
func (m *Machine) EnvKeys(containers []corev1.Container) []string {
	envKeys := []string{}
	for _, c := range containers {
		for _, e := range c.Env {
			envKeys = append(envKeys, e.Name)
		}
	}
	return envKeys
}
