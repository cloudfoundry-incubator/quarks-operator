package manifest

import (
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
)

const (
	// VarInterpolationContainerName is the name of the container that performs
	// variable interpolation for a manifest
	VarInterpolationContainerName = "var-interpolation"
)

var (
	// DockerOrganization is the organization which provides the operator image
	DockerOrganization = ""
	// DockerRepository is the repository which provides the operator image
	DockerRepository = ""
	// DockerTag is the tag of the operator image
	DockerTag = ""
	// LabelDeploymentName is the name of a label for the deployment name
	LabelDeploymentName = fmt.Sprintf("%s/deployment-name", apis.GroupName)
	// LabelInstanceGroupName is the name of a label for an instance group name
	LabelInstanceGroupName = fmt.Sprintf("%s/instance-group-name", apis.GroupName)
)

// KubeConfig represents a Manifest in kube resources
type KubeConfig struct {
	Variables                []esv1.ExtendedSecret
	InstanceGroups           []essv1.ExtendedStatefulSet
	Errands                  []ejv1.ExtendedJob
	Namespace                string
	VariableInterpolationJob *ejv1.ExtendedJob
	DataGatheringJob         *ejv1.ExtendedJob
}

// ConvertToKube converts a Manifest into kube resources
func (m *Manifest) ConvertToKube(namespace string) (KubeConfig, error) {
	kubeConfig := KubeConfig{
		Namespace: namespace,
	}

	convertedExtSts, err := m.convertToExtendedSts(namespace)
	if err != nil {
		return KubeConfig{}, err
	}

	convertedExtJob, err := m.convertToExtendedJob(namespace)
	if err != nil {
		return KubeConfig{}, err
	}

	dataGatheringJob, err := m.dataGatheringJob(namespace)
	if err != nil {
		return KubeConfig{}, err
	}

	varInterpolationJob, err := m.variableInterpolationJob(namespace)
	if err != nil {
		return KubeConfig{}, err
	}

	kubeConfig.Variables = m.convertVariables(namespace)
	kubeConfig.InstanceGroups = convertedExtSts
	kubeConfig.Errands = convertedExtJob
	kubeConfig.VariableInterpolationJob = varInterpolationJob
	kubeConfig.DataGatheringJob = dataGatheringJob

	return kubeConfig, nil
}

// generateVolumeName generate volume name based on secret name
func generateVolumeName(secretName string) string {
	nameSlices := strings.Split(secretName, ".")
	volName := ""
	if len(nameSlices) > 1 {
		volName = nameSlices[1]
	} else {
		volName = nameSlices[0]
	}
	return volName
}

// variableInterpolationJob returns an extended job to interpolate variables
func (m *Manifest) variableInterpolationJob(namespace string) (*ejv1.ExtendedJob, error) {
	cmd := []string{"cf-operator"}
	args := []string{"variable-interpolation"}

	// This is the source manifest, that still has the '((vars))'
	manifestSecretName := m.CalculateSecretName(DeploymentSecretTypeManifestWithOps, "")

	// Prepare Volumes and Volume mounts

	// This is a volume for the "not interpolated" manifest,
	// that has the ops files applied, but still contains '((vars))'
	volumes := []v1.Volume{
		{
			Name: generateVolumeName(manifestSecretName),
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: manifestSecretName,
				},
			},
		},
	}
	// Volume mount for the manifest
	volumeMounts := []v1.VolumeMount{
		{
			Name:      generateVolumeName(manifestSecretName),
			MountPath: "/var/run/secrets",
			ReadOnly:  true,
		},
	}

	// We need a volume and a mount for each input variable
	for _, variable := range m.Variables {
		varName := variable.Name
		varSecretName := m.CalculateSecretName(DeploymentSecretTypeGeneratedVariable, varName)

		// The volume definition
		vol := v1.Volume{
			Name: generateVolumeName(varSecretName),
			VolumeSource: v1.VolumeSource{
				Secret: &v1.SecretVolumeSource{
					SecretName: varSecretName,
				},
			},
		}
		volumes = append(volumes, vol)

		// And the volume mount
		volMount := v1.VolumeMount{
			Name:      generateVolumeName(varSecretName),
			MountPath: "/var/run/secrets/variables/" + varName,
			ReadOnly:  true,
		}
		volumeMounts = append(volumeMounts, volMount)
	}

	// Calculate the signature of the manifest, to label things
	manifestSignature, err := m.SHA1()
	if err != nil {
		return nil, errors.Wrap(err, "could not calculate manifest SHA1")
	}

	outputSecretPrefix, _ := m.CalculateEJobOutputSecretPrefixAndName(DeploymentSecretTypeManifestAndVars, VarInterpolationContainerName)

	// Assemble the Extended Job
	job := &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "variables-interpolation-job",
			Namespace: namespace,
			Labels: map[string]string{
				bdv1.LabelKind:       "variable-interpolation",
				bdv1.LabelDeployment: m.Name,
			},
		},
		Spec: ejv1.ExtendedJobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
					Containers: []v1.Container{
						{
							Name:         VarInterpolationContainerName,
							Image:        GetOperatorDockerImage(),
							Command:      cmd,
							Args:         args,
							VolumeMounts: volumeMounts,
							Env: []v1.EnvVar{
								{
									Name:  "MANIFEST",
									Value: "/var/run/secrets/manifest.yaml",
								},
								{
									Name:  "VARIABLES_DIR",
									Value: "/var/run/secrets/variables/",
								},
								{
									Name:  "FORMAT",
									Value: "encode",
								},
								{
									Name:  "ENCODE_KEY",
									Value: bdv1.InterpolatedManifestKey,
								},
							},
						},
					},
					Volumes: volumes,
				},
			},
			Output: &ejv1.Output{
				NamePrefix: outputSecretPrefix,
				SecretLabels: map[string]string{
					bdv1.LabelKind:         "desired-manifest",
					bdv1.LabelDeployment:   m.Name,
					bdv1.LabelManifestSHA1: manifestSignature,
				},
			},
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
		},
	}
	return job, nil
}

// SHA1 calculates the SHA1 of the manifest
func (m *Manifest) SHA1() (string, error) {
	manifestBytes, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", sha1.Sum(manifestBytes)), nil
}

// dataGatheringJob generates the Data Gathering Job for a manifest
func (m *Manifest) dataGatheringJob(namespace string) (*ejv1.ExtendedJob, error) {

	_, interpolatedManifestSecretName := m.CalculateEJobOutputSecretPrefixAndName(DeploymentSecretTypeManifestAndVars, VarInterpolationContainerName)

	eJobName := fmt.Sprintf("data-gathering-%s", m.Name)
	outputSecretNamePrefix, _ := m.CalculateEJobOutputSecretPrefixAndName(DeploymentSecretTypeInstanceGroupResolvedProperties, "")

	initContainers := []v1.Container{}
	containers := make([]v1.Container, len(m.InstanceGroups))

	doneSpecCopyingReleases := map[string]bool{}

	for idx, ig := range m.InstanceGroups {

		// Iterate through each Job to find all releases so we can copy all
		// sources to /var/vcap/data-gathering
		for _, boshJob := range ig.Jobs {
			// If we've already generated an init container for this release, skip
			releaseName := boshJob.Release
			if _, ok := doneSpecCopyingReleases[releaseName]; ok {
				continue
			}
			doneSpecCopyingReleases[releaseName] = true

			// Get the docker image for the release
			releaseImage, err := m.GetReleaseImage(ig.Name, boshJob.Name)
			if err != nil {
				return nil, errors.Wrap(err, "failed to calculate release image for data gathering")
			}

			// Create an init container that copies sources
			// TODO: destination should also contain release name, to prevent overwrites
			initContainers = append(initContainers, v1.Container{
				Name:  fmt.Sprintf("spec-copier-%s", releaseName),
				Image: releaseImage,
				VolumeMounts: []v1.VolumeMount{
					{Name: "data-gathering", MountPath: "/var/vcap/data-gathering"},
				},
				Command: []string{"bash", "-c", "cp -ar /var/vcap/jobs-src /var/vcap/data-gathering"},
			})
		}

		// One container per Instance Group
		// There will be one secret generated for each of these containers
		containers[idx] = v1.Container{
			Name:    ig.Name,
			Image:   GetOperatorDockerImage(),
			Command: []string{"/bin/sh"},
			Args:    []string{"-c", `cf-operator data-gather | base64 | tr -d '\n' | echo "{\"properties.yaml\":\"$(</dev/stdin)\"}"`},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      generateVolumeName(interpolatedManifestSecretName),
					MountPath: "/var/run/secrets",
					ReadOnly:  true,
				},
			},
			Env: []v1.EnvVar{
				{
					Name:  "BOSH_MANIFEST",
					Value: "/var/run/secrets/manifest.yml",
				},
				{
					Name:  "KUBERNETES_NAMESPACE",
					Value: namespace,
				},
				{
					Name:  "BASE_DIR",
					Value: "/var/vcap/data-gathering",
				},
				{
					Name:  "INSTANCE_GROUP",
					Value: ig.Name,
				},
			},
		}
	}

	// Construct the data gathering job
	dataGatheringJob := &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eJobName,
			Namespace: namespace,
		},
		Spec: ejv1.ExtendedJobSpec{
			Output: &ejv1.Output{
				NamePrefix: outputSecretNamePrefix,
				SecretLabels: map[string]string{
					LabelDeploymentName: m.Name,
				},
			},
			UpdateOnConfigChange: true,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: eJobName,
				},
				Spec: v1.PodSpec{
					// Init Container to copy contents
					InitContainers: initContainers,
					// Container to run data gathering
					Containers: containers,
					// Volumes for secrets
					Volumes: []v1.Volume{
						{
							Name: generateVolumeName(interpolatedManifestSecretName),
							VolumeSource: v1.VolumeSource{
								Secret: &v1.SecretVolumeSource{
									SecretName: interpolatedManifestSecretName,
								},
							},
						},
					},
				},
			},
		},
	}

	return dataGatheringJob, nil
}

// jobsToInitContainers creates a list of Containers for v1.PodSpec InitContainers field
func (m *Manifest) jobsToInitContainers(igName string, jobs []Job, namespace string) ([]v1.Container, error) {
	initContainers := []v1.Container{}

	// one init container for each release, for copying specs
	doneReleases := map[string]bool{}
	for _, job := range jobs {
		if _, ok := doneReleases[job.Release]; ok {
			continue
		}

		doneReleases[job.Release] = true
		releaseImage, err := m.GetReleaseImage(igName, job.Name)
		if err != nil {
			return []v1.Container{}, err
		}

		initContainers = append(initContainers, v1.Container{
			Name:  fmt.Sprintf("spec-copier-%s", job.Name),
			Image: releaseImage,
			VolumeMounts: []v1.VolumeMount{
				{Name: "rendering-data", MountPath: "/var/vcap/rendering"},
			},
			Command: []string{"bash", "-c", "cp -ar /var/vcap/jobs-src /var/vcap/rendering"},
		})
	}

	initContainers = append(initContainers, v1.Container{
		Name:  fmt.Sprintf("renderer-%s", igName),
		Image: GetOperatorDockerImage(),
		VolumeMounts: []v1.VolumeMount{
			{Name: "rendering-data", MountPath: "/var/vcap/rendering"},
		},
		Command: []string{"bash", "-c", "echo 'foo'"},
	})

	return initContainers, nil
}

// jobsToContainers creates a list of Containers for v1.PodSpec Containers field
func (m *Manifest) jobsToContainers(igName string, jobs []Job, namespace string) ([]v1.Container, error) {
	var jobsToContainerPods []v1.Container

	if len(jobs) == 0 {
		return nil, fmt.Errorf("instance group %s has no jobs defined", igName)
	}

	for _, job := range jobs {
		jobImage, err := m.GetReleaseImage(igName, job.Name)
		if err != nil {
			return []v1.Container{}, err
		}
		jobsToContainerPods = append(jobsToContainerPods, v1.Container{
			Name:  fmt.Sprintf("job-%s", job.Name),
			Image: jobImage,
			VolumeMounts: []v1.VolumeMount{
				{Name: "rendering-data", MountPath: "/var/vcap/rendering"},
			},
			Command: []string{"bash", "-c", "sleep 3600"},
		})
	}
	return jobsToContainerPods, nil
}

// serviceToExtendedSts will generate an ExtendedStatefulSet
func (m *Manifest) serviceToExtendedSts(ig *InstanceGroup, namespace string) (essv1.ExtendedStatefulSet, error) {
	igName := ig.Name

	listOfContainers, err := m.jobsToContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	listOfInitContainers, err := m.jobsToInitContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	extSts := essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", m.Name, igName),
			Namespace: namespace,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: v1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: igName,
				},
				Spec: v1beta2.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(ig.Instances); return &i }(),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							LabelDeploymentName:    m.Name,
							LabelInstanceGroupName: igName,
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: igName,
							Labels: map[string]string{
								LabelDeploymentName:    m.Name,
								LabelInstanceGroupName: igName,
							},
						},
						Spec: v1.PodSpec{
							Volumes: []v1.Volume{
								{
									Name:         "rendering-data",
									VolumeSource: v1.VolumeSource{EmptyDir: &v1.EmptyDirVolumeSource{}},
								},
							},
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
func (m *Manifest) convertToExtendedSts(namespace string) ([]essv1.ExtendedStatefulSet, error) {
	extStsList := []essv1.ExtendedStatefulSet{}
	for _, ig := range m.InstanceGroups {
		if ig.LifeCycle == "service" || ig.LifeCycle == "" {
			convertedExtStatefulSet, err := m.serviceToExtendedSts(ig, namespace)
			if err != nil {
				return []essv1.ExtendedStatefulSet{}, err
			}
			extStsList = append(extStsList, convertedExtStatefulSet)
		}
	}
	return extStsList, nil
}

// errandToExtendedJob will generate an ExtendedJob
func (m *Manifest) errandToExtendedJob(ig *InstanceGroup, namespace string) (ejv1.ExtendedJob, error) {
	igName := ig.Name

	listOfContainers, err := m.jobsToContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}
	listOfInitContainers, err := m.jobsToInitContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}
	extJob := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", m.Name, igName),
			Namespace: namespace,
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
func (m *Manifest) convertToExtendedJob(namespace string) ([]ejv1.ExtendedJob, error) {
	extJobs := []ejv1.ExtendedJob{}
	for _, ig := range m.InstanceGroups {
		if ig.LifeCycle == "errand" {
			convertedExtJob, err := m.errandToExtendedJob(ig, namespace)
			if err != nil {
				return []ejv1.ExtendedJob{}, err
			}
			extJobs = append(extJobs, convertedExtJob)
		}
	}
	return extJobs, nil
}

func (m *Manifest) convertVariables(namespace string) []esv1.ExtendedSecret {
	secrets := []esv1.ExtendedSecret{}

	for _, v := range m.Variables {
		secretName := m.CalculateSecretName(DeploymentSecretTypeGeneratedVariable, v.Name)
		s := esv1.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Labels: map[string]string{
					"variableName": v.Name,
				},
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
					Name: m.CalculateSecretName(DeploymentSecretTypeGeneratedVariable, v.Options.CA),
					Key:  "certificate",
				}
				certRequest.CAKeyRef = esv1.SecretReference{
					Name: m.CalculateSecretName(DeploymentSecretTypeGeneratedVariable, v.Options.CA),
					Key:  "private_key",
				}
			}
			s.Spec.Request.CertificateRequest = certRequest
		}
		secrets = append(secrets, s)
	}

	return secrets
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
		return "", fmt.Errorf("instance group '%s' not found", instanceGroupName)
	}

	var stemcell *Stemcell
	for i := range m.Stemcells {
		if m.Stemcells[i].Alias == instanceGroup.Stemcell {
			stemcell = m.Stemcells[i]
		}
	}

	var job *Job
	for i := range instanceGroup.Jobs {
		if instanceGroup.Jobs[i].Name == jobName {
			job = &instanceGroup.Jobs[i]
			break
		}
	}
	if job == nil {
		return "", fmt.Errorf("job '%s' not found in instance group '%s'", jobName, instanceGroupName)
	}

	for i := range m.Releases {
		if m.Releases[i].Name == job.Release {
			release := m.Releases[i]
			name := strings.TrimRight(release.URL, "/")

			var stemcellVersion string

			if release.Stemcell != nil {
				stemcellVersion = release.Stemcell.OS + "-" + release.Stemcell.Version
			} else {
				if stemcell == nil {
					return "", fmt.Errorf("stemcell could not be resolved for instance group %s", instanceGroup.Name)
				}
				stemcellVersion = stemcell.OS + "-" + stemcell.Version
			}
			return fmt.Sprintf("%s/%s:%s-%s", name, release.Name, stemcellVersion, release.Version), nil
		}
	}
	return "", fmt.Errorf("release '%s' not found", job.Release)
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return DockerOrganization + "/" + DockerRepository + ":" + DockerTag
}
