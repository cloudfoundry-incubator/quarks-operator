package environment

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// CreateExtendedSecret creates a ExtendedSecret custom resource and returns a function to delete it
func (m *Machine) CreateExtendedSecret(namespace string, es esv1.ExtendedSecret) (*esv1.ExtendedSecret, machine.TearDownFunc, error) {
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
