package environment

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

// WaitForServiceVersion blocks until the service of the instance group is created/updated. It fails after the timeout.
func (m *Machine) WaitForServiceVersion(namespace string, serviceName string, version string) error {
	return wait.PollImmediate(m.PollInterval, m.PollTimeout, func() (bool, error) {
		svc, err := m.Clientset.CoreV1().Services(namespace).Get(context.Background(), serviceName, metav1.GetOptions{})
		if err != nil {
			return false, errors.Wrapf(err, "failed to get service '%s'", svc)
		}

		deploymentVersion, ok := svc.Labels[bdv1.LabelDeploymentVersion]
		if ok && deploymentVersion == version {
			return true, nil
		}

		return false, nil
	})
}
