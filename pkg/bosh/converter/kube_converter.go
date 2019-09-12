package converter

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	certv1 "k8s.io/api/certificates/v1beta1"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

// KubeConverter represents a Manifest in kube resources
type KubeConverter struct {
	namespace string
}

// NewKubeConverter converts a Manifest into kube resources
func NewKubeConverter(namespace string) *KubeConverter {
	return &KubeConverter{
		namespace: namespace,
	}
}

// Variables returns extended secrets for a list of BOSH variables
func (kc *KubeConverter) Variables(manifestName string, variables []bdm.Variable) ([]esv1.ExtendedSecret, error) {
	secrets := []esv1.ExtendedSecret{}

	for _, v := range variables {
		secretName := names.CalculateSecretName(names.DeploymentSecretTypeVariable, manifestName, v.Name)
		s := esv1.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: kc.namespace,
				Labels: map[string]string{
					"variableName":          v.Name,
					bdm.LabelDeploymentName: manifestName,
				},
			},
			Spec: esv1.ExtendedSecretSpec{
				Type:       v.Type,
				SecretName: secretName,
			},
		}
		if v.Type == esv1.Certificate {
			if v.Options == nil {
				return secrets, fmt.Errorf("Invalid certificate ExtendedSecret: missing options key")
			}

			usages := []certv1.KeyUsage{}

			for _, keyUsage := range v.Options.ExtendedKeyUsage {
				if keyUsage == bdm.ClientAuth {
					usages = append(usages, certv1.UsageClientAuth)
				}
				if keyUsage == bdm.ServerAuth {
					usages = append(usages, certv1.UsageServerAuth)
				}
			}

			if v.Options.IsCA {
				usages = append(usages,
					certv1.UsageSigning,
					certv1.UsageDigitalSignature,
					certv1.UsageAny,
					certv1.UsageCertSign,
					certv1.UsageCodeSigning,
					certv1.UsageDigitalSignature)
			}

			certRequest := esv1.CertificateRequest{
				CommonName:       v.Options.CommonName,
				AlternativeNames: v.Options.AlternativeNames,
				IsCA:             v.Options.IsCA,
				SignerType:       v.Options.SignerType,
				ServiceRef:       v.Options.ServiceRef,
				Usages:           usages,
			}
			if len(certRequest.SignerType) == 0 {
				certRequest.SignerType = esv1.LocalSigner
			}
			if v.Options.CA != "" {
				certRequest.CARef = esv1.SecretReference{
					Name: names.CalculateSecretName(names.DeploymentSecretTypeVariable, manifestName, v.Options.CA),
					Key:  "certificate",
				}
				certRequest.CAKeyRef = esv1.SecretReference{
					Name: names.CalculateSecretName(names.DeploymentSecretTypeVariable, manifestName, v.Options.CA),
					Key:  "private_key",
				}
			}
			s.Spec.Request.CertificateRequest = certRequest
		}
		secrets = append(secrets, s)
	}

	return secrets, nil
}
