package converter

import (
	"fmt"

	certv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
	qsv1a1 "code.cloudfoundry.org/quarks-secret/pkg/kube/apis/quarkssecret/v1alpha1"
)

// VariablesConverter represents a BOSH manifest into kubernetes resources
type VariablesConverter struct {
}

// NewVariablesConverter converts a BOSH manifest into kubernetes resources
func NewVariablesConverter() *VariablesConverter {
	return &VariablesConverter{}
}

// Variables returns quarks secrets for a list of BOSH variables
func (vc *VariablesConverter) Variables(namespace string, manifestName string, variables []bdm.Variable) ([]qsv1a1.QuarksSecret, error) {
	secrets := []qsv1a1.QuarksSecret{}

	for _, v := range variables {
		secretName := names.SecretVariableName(v.Name)
		s := qsv1a1.QuarksSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Labels: map[string]string{
					"variableName":           v.Name,
					bdv1.LabelDeploymentName: manifestName,
				},
			},
			Spec: qsv1a1.QuarksSecretSpec{
				Type:       v.Type,
				SecretName: secretName,
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName: manifestName,
				},
			},
		}

		if v.Options != nil {
			s.Spec.Copies = v.Options.Copies
		}

		if v.Type == qsv1a1.Certificate {
			if v.Options == nil {
				return secrets, fmt.Errorf("invalid certificate QuarksSecret: missing options key")
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

			certRequest := qsv1a1.CertificateRequest{
				CommonName:                  v.Options.CommonName,
				AlternativeNames:            v.Options.AlternativeNames,
				IsCA:                        v.Options.IsCA,
				SignerType:                  v.Options.SignerType,
				ServiceRef:                  v.Options.ServiceRef,
				ActivateEKSWorkaroundForSAN: v.Options.ActivateEKSWorkaroundForSAN,
				Usages:                      usages,
			}
			if len(certRequest.SignerType) == 0 {
				certRequest.SignerType = qsv1a1.LocalSigner
			}
			if v.Options.CA != "" {
				certRequest.CARef = qsv1a1.SecretReference{
					Name: names.SecretVariableName(v.Options.CA),
					Key:  "certificate",
				}
				certRequest.CAKeyRef = qsv1a1.SecretReference{
					Name: names.SecretVariableName(v.Options.CA),
					Key:  "private_key",
				}
			}
			s.Spec.Request.CertificateRequest = certRequest
		}

		secrets = append(secrets, s)
	}

	return secrets, nil
}
