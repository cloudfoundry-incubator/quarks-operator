package environment

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

// CreateBOSHDeploymentUsingChan creates a BOSHDeployment custom resource and returns an error via a channel
func (m *Machine) CreateBOSHDeploymentUsingChan(outputChannel chan ChanResult, namespace string, deployment bdv1.BOSHDeployment) {
	client := m.VersionedClientset.BoshdeploymentV1alpha1().BOSHDeployments(namespace)
	_, err := client.Create(&deployment)
	outputChannel <- ChanResult{
		Error: err,
	}
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
