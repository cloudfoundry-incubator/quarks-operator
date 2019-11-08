package converter

import (
	"fmt"

	certv1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/disk"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// KubeConverter represents a Manifest in kube resources
type KubeConverter struct {
	namespace               string
	volumeFactory           VolumeFactory
	newContainerFactoryFunc NewContainerFactoryFunc
}

// ContainerFactory builds Kubernetes containers from BOSH jobs.
type ContainerFactory interface {
	JobsToInitContainers(jobs []bdm.Job, defaultVolumeMounts []corev1.VolumeMount, bpmDisks disk.BPMResourceDisks, requiredService *string) ([]corev1.Container, error)
	JobsToContainers(jobs []bdm.Job, defaultVolumeMounts []corev1.VolumeMount, bpmDisks disk.BPMResourceDisks) ([]corev1.Container, error)
}

// NewContainerFactoryFunc returns ContainerFactory from single BOSH instance group.
type NewContainerFactoryFunc func(manifestName string, instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider ReleaseImageProvider, bpmConfigs bpm.Configs) ContainerFactory

// VolumeFactory builds Kubernetes containers from BOSH jobs.
type VolumeFactory interface {
	GenerateDefaultDisks(manifestName string, instanceGroupName string, igResolvedSecretVersion string, namespace string) disk.BPMResourceDisks
	GenerateBPMDisks(manifestName string, instanceGroup *bdm.InstanceGroup, bpmConfigs bpm.Configs, namespace string) (disk.BPMResourceDisks, error)
}

// NewKubeConverter converts a Manifest into kube resources
func NewKubeConverter(namespace string, volumeFactory VolumeFactory, newContainerFactoryFunc NewContainerFactoryFunc) *KubeConverter {
	return &KubeConverter{
		namespace:               namespace,
		volumeFactory:           volumeFactory,
		newContainerFactoryFunc: newContainerFactoryFunc,
	}
}

// Variables returns quarks secrets for a list of BOSH variables
func (kc *KubeConverter) Variables(manifestName string, variables []bdm.Variable) ([]qsv1a1.QuarksSecret, error) {
	secrets := []qsv1a1.QuarksSecret{}

	for _, v := range variables {
		secretName := names.CalculateSecretName(names.DeploymentSecretTypeVariable, manifestName, v.Name)
		s := qsv1a1.QuarksSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: kc.namespace,
				Labels: map[string]string{
					"variableName":          v.Name,
					bdm.LabelDeploymentName: manifestName,
				},
			},
			Spec: qsv1a1.QuarksSecretSpec{
				Type:       v.Type,
				SecretName: secretName,
			},
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
					Name: names.CalculateSecretName(names.DeploymentSecretTypeVariable, manifestName, v.Options.CA),
					Key:  "certificate",
				}
				certRequest.CAKeyRef = qsv1a1.SecretReference{
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
