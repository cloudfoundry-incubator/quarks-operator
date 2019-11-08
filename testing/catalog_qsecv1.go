package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
)

// DefaultQuarksSecret for use in tests
func (c *Catalog) DefaultQuarksSecret(name string) qsv1a1.QuarksSecret {
	return qsv1a1.QuarksSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qsv1a1.QuarksSecretSpec{
			Type:       "password",
			SecretName: "generated-secret",
		},
	}
}
