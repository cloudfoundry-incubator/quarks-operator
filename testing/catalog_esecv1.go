package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esecv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
)

// DefaultExtendedSecret for use in tests
func (c *Catalog) DefaultExtendedSecret(name string) esecv1.ExtendedSecret {
	return esecv1.ExtendedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esecv1.ExtendedSecretSpec{
			Type:       "password",
			SecretName: "generated-secret",
		},
	}
}
