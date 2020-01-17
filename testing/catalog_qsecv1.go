package testing

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
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

// RotationConfig is a config map, which triggers secret rotation
func (c *Catalog) RotationConfig(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "rotation-config1",
			Labels: map[string]string{
				qsv1a1.LabelSecretRotationTrigger: "yes",
			},
		},
		Data: map[string]string{
			qsv1a1.RotateQSecretListName: fmt.Sprintf(`["%s"]`, name),
		},
	}
}
