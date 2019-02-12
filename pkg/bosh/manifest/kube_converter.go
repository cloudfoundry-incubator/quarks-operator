package manifest

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strings"

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

	for _, v := range m.Variables {
		secretName := m.generateVariableSecretName(v.Name)
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

func (m *Manifest) generateVariableSecretName(name string) string {
	nameRegex := regexp.MustCompile("[^-][a-z0-9-]*.[a-z0-9-]*[^-]")
	partRegex := regexp.MustCompile("[a-z0-9-]*")

	deploymentName := partRegex.FindString(strings.Replace(m.Name, "_", "-", -1))
	variableName := partRegex.FindString(strings.Replace(name, "_", "-", -1))
	secretName := nameRegex.FindString(deploymentName + "." + variableName)

	if len(secretName) > 63 {
		// secret names are limited to 63 characters so we recalculate the name as
		// <name trimmed to 31 characters><md5 hash of name>
		sumHex := md5.Sum([]byte(secretName))
		sum := hex.EncodeToString(sumHex[:])
		secretName = secretName[:63-32] + sum
	}

	return secretName
}
