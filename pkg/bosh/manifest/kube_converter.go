package manifest

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/version"
	"k8s.io/api/apps/v1beta2"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// DockerOrganization is the organization which provides the operator image
	DockerOrganization = ""
	// DockerRepository is the repository which provides the operator image
	DockerRepository = ""
)

// KubeConfig represents a Manifest in kube resources
type KubeConfig struct {
	Variables   []esv1.ExtendedSecret
	ExtendedSts []essv1.ExtendedStatefulSet
	ExtendedJob []ejv1.ExtendedJob
}

// ConvertToKube converts a Manifest into kube resources
func (m *Manifest) ConvertToKube() (KubeConfig, error) {
	kubeConfig := KubeConfig{}

	convertedExtSts, err := m.convertToExtendedSts()
	if err != nil {
		return KubeConfig{}, err
	}

	convertedExtJob, err := m.convertToExtendedJob()
	if err != nil {
		return KubeConfig{}, err
	}
	kubeConfig.Variables = m.convertVariables()
	kubeConfig.ExtendedSts = convertedExtSts
	kubeConfig.ExtendedJob = convertedExtJob

	return kubeConfig, nil
}

// jobsToInitContainers creates a list of Containers for v1.PodSpec InitContainers field
func (m *Manifest) jobsToInitContainers(instanceName string, jobs []Job) ([]v1.Container, error) {

	var initContainers []v1.Container
	initContainers, err := m.jobsToContainers(instanceName, jobs, true)
	if err != nil {
		return []v1.Container{}, err
	}
	initContainers = append([]v1.Container{{Name: instanceName, Image: GetOperatorDockerImage()}}, initContainers...) //TODO: name of the first init container?
	return initContainers, nil
}

// jobsToContainers creates a list of Containers for v1.PodSpec Containers field
func (m *Manifest) jobsToContainers(igName string, jobs []Job, isInitContainer bool) ([]v1.Container, error) {
	var (
		jobsToContainerPods []v1.Container
		containerCmd        string
	)
	if isInitContainer {
		containerCmd = "echo \"\""
	} else {
		containerCmd = "while true; do ping localhost;done"
	}

	for _, job := range jobs {
		jobImage, err := m.GetReleaseImage(igName, job.Name)
		if err != nil {
			return []v1.Container{}, err
		}
		jobsToContainerPods = append(jobsToContainerPods, v1.Container{
			Name:    job.Name,
			Image:   jobImage,
			Command: []string{containerCmd},
		})
	}
	return jobsToContainerPods, nil
}

// serviceToExtendedSts will generate an ExtendedStatefulSet
func (m *Manifest) serviceToExtendedSts(ig *InstanceGroup) (essv1.ExtendedStatefulSet, error) {

	igName := ig.Name

	listOfContainers, err := m.jobsToContainers(igName, ig.Jobs, false)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	listOfInitContainers, err := m.jobsToInitContainers(igName, ig.Jobs)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	extSts := essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: igName,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: v1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: igName,
				},
				Spec: v1beta2.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(ig.Instances); return &i }(),
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: igName,
						},
						Spec: v1.PodSpec{
							Containers:     listOfContainers,
							InitContainers: listOfInitContainers,
						},
					},
				},
			},
		},
	}
	return extSts, nil
}

// convertToExtendedSts will convert instance_groups which lifecycle
// is service to ExtendedStatefulSets
func (m *Manifest) convertToExtendedSts() ([]essv1.ExtendedStatefulSet, error) {
	extStsList := []essv1.ExtendedStatefulSet{}
	for _, ig := range m.InstanceGroups {
		if ig.LifeCycle == "service" || ig.LifeCycle == "" {
			convertedExtStatefulSet, err := m.serviceToExtendedSts(ig)
			if err != nil {
				return []essv1.ExtendedStatefulSet{}, err
			}
			extStsList = append(extStsList, convertedExtStatefulSet)
		}
	}
	return extStsList, nil
}

// errandToExtendedJob will generate an ExtendedJob
func (m *Manifest) errandToExtendedJob(ig *InstanceGroup) (ejv1.ExtendedJob, error) {
	igName := ig.Name

	listOfContainers, err := m.jobsToContainers(igName, ig.Jobs, false)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}
	listOfInitContainers, err := m.jobsToInitContainers(igName, ig.Jobs)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}
	extJob := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name: igName,
		},
		Spec: ejv1.ExtendedJobSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: igName,
				},
				Spec: v1.PodSpec{
					Containers:     listOfContainers,
					InitContainers: listOfInitContainers,
				},
			},
		},
	}
	return extJob, nil
}

// convertToExtendedJob will convert instance_groups which lifecycle is
// errand to ExtendedJobs
func (m *Manifest) convertToExtendedJob() ([]ejv1.ExtendedJob, error) {
	extJobs := []ejv1.ExtendedJob{}
	for _, ig := range m.InstanceGroups {
		if ig.LifeCycle == "errand" {
			convertedExtJob, err := m.errandToExtendedJob(ig)
			if err != nil {
				return []ejv1.ExtendedJob{}, err
			}
			extJobs = append(extJobs, convertedExtJob)
		}
	}
	return extJobs, nil
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

// GetReleaseImage returns the release image location for a given instance group/job
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

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return DockerOrganization + "/" + DockerRepository + ":" + version.Version
}
