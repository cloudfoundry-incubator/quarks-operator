package manifest

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
)

// KubeConfig represents a Manifest in kube resources
type KubeConfig struct {
	Variables []esv1.ExtendedSecret
}

// ConvertToKube converts a Manifest into kube resources
func (m *Manifest) ConvertToKube() KubeConfig {
	kubeConfig := KubeConfig{}

	kubeConfig.Variables = m.convertVariables()

	return kubeConfig
}

func (m *Manifest) convertVariables() []esv1.ExtendedSecret {
	secrets := []esv1.ExtendedSecret{}

	deploymentName := m.Name
	for _, v := range m.Variables {
		secretName := deploymentName + "." + v.Name
		s := esv1.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name: secretName,
			},
			Spec: esv1.ExtendedSecretSpec{
				Type:       esv1.Type(v.Type),
				SecretName: secretName,
			},
		}
		secrets = append(secrets, s)
	}

	return secrets
}
