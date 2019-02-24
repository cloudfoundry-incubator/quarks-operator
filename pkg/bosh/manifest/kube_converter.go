package manifest

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
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
		if esv1.Type(v.Type) == esv1.Certificate {
			certRequest := esv1.CertificateRequest{
				CommonName:       v.Options.CommonName,
				AlternativeNames: v.Options.AlternativeNames,
				IsCA:             v.Options.IsCA,
			}
			if v.Options.CA != "" {
				certRequest.CARef = esv1.SecretReference{
					Name: m.generateVariableSecretName(v.Options.CA),
					Key:  "certificate",
				}
			}
			s.Spec.Request.CertificateRequest = certRequest
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

func (m *Manifest) GetReleaseImage(instanceGroupName, jobName string) (string, error) {
	var instanceGroup *InstanceGroup
	for i := range m.InstanceGroups {
		if m.InstanceGroups[i].Name == instanceGroupName {
			instanceGroup = m.InstanceGroups[i]
			break
		}
	}
	if instanceGroup == nil {
		return "", fmt.Errorf("Instance group '%s' not found", instanceGroupName)
	}

	var stemcell *Stemcell
	for i := range m.Stemcells {
		if m.Stemcells[i].Alias == instanceGroup.Stemcell {
			stemcell = m.Stemcells[i]
		}
	}
	if stemcell == nil {
		return "", fmt.Errorf("Stemcell '%s' not found", instanceGroup.Stemcell)
	}

	var job *Job
	for i := range instanceGroup.Jobs {
		if instanceGroup.Jobs[i].Name == jobName {
			job = &instanceGroup.Jobs[i]
			break
		}
	}
	if job == nil {
		return "", fmt.Errorf("Job '%s' not found in instance group '%s'", jobName, instanceGroupName)
	}

	for i := range m.Releases {
		if m.Releases[i].Name == job.Release {
			release := m.Releases[i]
			name := strings.TrimRight(release.URL, "/")

			stemcellVersion := stemcell.OS + "-" + stemcell.Version
			if release.Stemcell != nil {
				stemcellVersion = release.Stemcell.OS + "-" + release.Stemcell.Version
			}
			return name + "/" + release.Name + "-release:" + stemcellVersion + "-" + release.Version, nil
		}
	}
	return "", fmt.Errorf("Release '%s' not found", job.Release)
}
