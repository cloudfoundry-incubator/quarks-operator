package environment

import (
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// WaitForBOSHDeploymentDeletion blocks until the CR is deleted
func (m *Machine) WaitForBOSHDeploymentDeletion(namespace string, name string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
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

// CreateBOSHDeploymentUsingChan creates a BOSHDeployment custom resource and returns an error via a channel
func (m *Machine) CreateBOSHDeploymentUsingChan(outputChannel chan machine.ChanResult, namespace string, deployment bdv1.BOSHDeployment) {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	_, err := client.Create(&deployment)
	outputChannel <- machine.ChanResult{
		Error: err,
	}
}

// CreateBOSHDeployment creates a BOSHDeployment custom resource and returns a function to delete it
func (m *Machine) CreateBOSHDeployment(namespace string, deployment bdv1.BOSHDeployment) (*bdv1.BOSHDeployment, machine.TearDownFunc, error) {
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
func (m *Machine) UpdateBOSHDeployment(namespace string, deployment bdv1.BOSHDeployment) (*bdv1.BOSHDeployment, machine.TearDownFunc, error) {
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
