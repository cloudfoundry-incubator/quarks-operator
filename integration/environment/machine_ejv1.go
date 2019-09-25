package environment

import (
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
)

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
