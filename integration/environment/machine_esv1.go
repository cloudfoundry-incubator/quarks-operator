package environment

import (
	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// GetStatefulSet gets a StatefulSet custom resource
func (m *Machine) GetStatefulSet(namespace string, name string) (*v1beta1.StatefulSet, error) {
	statefulSet, err := m.Clientset.AppsV1beta1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return &v1beta1.StatefulSet{}, errors.Wrapf(err, "failed to query for statefulSet by name: %v", name)
	}

	return statefulSet, nil
}

// CreateQuarksStatefulSet creates a QuarksStatefulSet custom resource and returns a function to delete it
func (m *Machine) CreateQuarksStatefulSet(namespace string, ess qstsv1a1.QuarksStatefulSet) (*qstsv1a1.QuarksStatefulSet, machine.TearDownFunc, error) {
	client := m.VersionedClientset.QuarksstatefulsetV1alpha1().QuarksStatefulSets(namespace)

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

// GetQuarksStatefulSet gets a QuarksStatefulSet custom resource
func (m *Machine) GetQuarksStatefulSet(namespace string, name string) (*qstsv1a1.QuarksStatefulSet, error) {
	client := m.VersionedClientset.QuarksstatefulsetV1alpha1().QuarksStatefulSets(namespace)
	d, err := client.Get(name, metav1.GetOptions{})
	return d, err
}

// UpdateQuarksStatefulSet updates a QuarksStatefulSet custom resource and returns a function to delete it
func (m *Machine) UpdateQuarksStatefulSet(namespace string, ess qstsv1a1.QuarksStatefulSet) (*qstsv1a1.QuarksStatefulSet, machine.TearDownFunc, error) {
	client := m.VersionedClientset.QuarksstatefulsetV1alpha1().QuarksStatefulSets(namespace)
	d, err := client.Update(&ess)
	return d, func() error {
		err := client.Delete(ess.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// DeleteQuarksStatefulSet deletes a QuarksStatefulSet custom resource
func (m *Machine) DeleteQuarksStatefulSet(namespace string, name string) error {
	client := m.VersionedClientset.QuarksstatefulsetV1alpha1().QuarksStatefulSets(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}

// WaitForStatefulSetDelete blocks until the specified statefulSet is deleted
func (m *Machine) WaitForStatefulSetDelete(namespace string, name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		found, err := m.StatefulSetExist(namespace, name)
		return !found, err
	})
}

// StatefulSetExist checks if the statefulSet exists
func (m *Machine) StatefulSetExist(namespace string, name string) (bool, error) {
	_, err := m.Clientset.AppsV1beta1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for statefulSet by name: %s", name)
	}
	return true, nil
}

// WaitForStatefulSetNewGeneration blocks until at least one StatefulSet is found. It fails after the timeout.
func (m *Machine) WaitForStatefulSetNewGeneration(namespace string, name string, currentVersion int64) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.StatefulSetNewGeneration(namespace, name, currentVersion)
	})
}

// WaitForQuarksStatefulSets blocks until at least one QuarksStatefulSet is found. It fails after the timeout.
func (m *Machine) WaitForQuarksStatefulSets(namespace string, labels string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.QuarksStatefulSetExists(namespace, labels)
	})
}

// WaitForPV blocks until the pv is running. It fails after the timeout.
func (m *Machine) WaitForPV(name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
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
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
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
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
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

// WaitForStatefulSet blocks until all statefulSet pods are running. It fails after the timeout.
func (m *Machine) WaitForStatefulSet(namespace string, labels string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		return m.StatefulSetRunning(namespace, labels)
	})
}

// StatefulSetRunning returns true if the statefulSet by that name has all pods created
func (m *Machine) StatefulSetRunning(namespace string, name string) (bool, error) {
	statefulSet, err := m.Clientset.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to query for statefulSet by name: %s", name)
	}

	if statefulSet.Status.ReadyReplicas == *statefulSet.Spec.Replicas {
		return true, nil
	}
	return false, nil
}

// QuarksStatefulSetExists returns true if at least one ess selected by labels exists
func (m *Machine) QuarksStatefulSetExists(namespace string, labels string) (bool, error) {
	esss, err := m.VersionedClientset.QuarksstatefulsetV1alpha1().QuarksStatefulSets(namespace).List(metav1.ListOptions{
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
