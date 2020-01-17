package environment

// The functions in this file are only used by the extended secret component
// tests.  They were split off in preparation for standalone components.

import (
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

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

// QuarksSecretChangedFunc returns true if something changed in the quarks secret
type QuarksSecretChangedFunc func(qsv1a1.QuarksSecret) bool

// WaitForQuarksSecretChange waits for the quarks secret to fulfill the change func
func (m *Machine) WaitForQuarksSecretChange(namespace string, name string, changed QuarksSecretChangedFunc) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		client := m.VersionedClientset.QuarkssecretV1alpha1().QuarksSecrets(namespace)
		qs, err := client.Get(name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, errors.Wrapf(err, "failed to query for quarks secret: %s", name)
		}

		return changed(*qs), nil
	})
}
