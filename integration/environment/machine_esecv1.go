package environment

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/testing/machine"
)

// CreateQuarksSecret creates a QuarksSecret custom resource and returns a function to delete it
func (m *Machine) CreateQuarksSecret(namespace string, qs qsv1a1.QuarksSecret) (*qsv1a1.QuarksSecret, machine.TearDownFunc, error) {
	client := m.VersionedClientset.QuarkssecretV1alpha1().QuarksSecrets(namespace)
	d, err := client.Create(&qs)
	return d, func() error {
		err := client.Delete(qs.GetName(), &metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, err
}

// DeleteQuarksSecret deletes an QuarksSecret custom resource
func (m *Machine) DeleteQuarksSecret(namespace string, name string) error {
	client := m.VersionedClientset.QuarkssecretV1alpha1().QuarksSecrets(namespace)
	return client.Delete(name, &metav1.DeleteOptions{})
}
