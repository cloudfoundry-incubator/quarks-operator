package converter

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

const (
	// EnvInstanceGroupName is a key for the container Env identifying the
	// instance group that container is started for
	EnvInstanceGroupName = "INSTANCE_GROUP_NAME"
	// EnvBOSHManifestPath is a key for the container Env pointing to the BOSH manifest
	EnvBOSHManifestPath = "BOSH_MANIFEST_PATH"
	// EnvCFONamespace is a key for the container Env used to lookup the
	// namespace CF operator is running in
	EnvCFONamespace = "CF_OPERATOR_NAMESPACE"
	// EnvBaseDir is a key for the container Env used to lookup the base dir
	EnvBaseDir = "BASE_DIR"
	// EnvVariablesDir is a key for the container Env used to lookup the variables dir
	EnvVariablesDir = "VARIABLES_DIR"
	// EnvOutputFilePath is path where json output is to be redirected
	EnvOutputFilePath = "OUTPUT_FILE_PATH"
	// EnvOutputFilePathValue is the value of filepath of json output file
	EnvOutputFilePathValue = "/mnt/quarks/output.json"
	// VarInterpolationContainerName is the name of the container that
	// performs variable interpolation for a manifest. It's also part of
	// the output secret's name
	VarInterpolationContainerName = "desired-manifest"
	// PodNameEnvVar is the environment variable containing metadata.name used to render BOSH spec.id.
	PodNameEnvVar = "POD_NAME"
	// PodIPEnvVar is the environment variable containing status.podIP used to render BOSH spec.ip.
	PodIPEnvVar = "POD_IP"
)

// JobFactory is a concrete implementation of JobFactory
type JobFactory struct {
	Namespace string
}

// NewJobFactory returns a concrete implementation of JobFactory
func NewJobFactory(namespace string) *JobFactory {
	return &JobFactory{
		Namespace: namespace,
	}
}

// VariableInterpolationJob returns an extended job to interpolate variables
func (f *JobFactory) VariableInterpolationJob(manifest bdm.Manifest) (*ejv1.ExtendedJob, error) {
	args := []string{"util", "variable-interpolation"}

	// This is the source manifest, that still has the '((vars))'
	manifestSecretName := names.CalculateSecretName(names.DeploymentSecretTypeManifestWithOps, manifest.Name, "")

	// Prepare Volumes and Volume mounts

	volumes := []corev1.Volume{*withOpsVolume(manifestSecretName)}
	volumeMounts := []corev1.VolumeMount{withOpsVolumeMount(manifestSecretName)}

	// We need a volume and a mount for each input variable
	for _, variable := range manifest.Variables {
		varName := variable.Name
		varSecretName := names.CalculateSecretName(names.DeploymentSecretTypeVariable, manifest.Name, varName)

		volumes = append(volumes, variableVolume(varSecretName))
		volumeMounts = append(volumeMounts, variableVolumeMount(varSecretName, varName))
	}

	// If there are no variables, mount an empty dir for variables
	if len(manifest.Variables) == 0 {
		volumes = append(volumes, noVarsVolume())
		volumeMounts = append(volumeMounts, noVarsVolumeMount())
	}

	eJobName := fmt.Sprintf("dm-%s", manifest.Name)

	// Construct the var interpolation auto-errand eJob
	job := &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eJobName,
			Namespace: f.Namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: manifest.Name,
			},
		},
		Spec: ejv1.ExtendedJobSpec{
			Output: &ejv1.Output{
				NamePrefix: names.DesiredManifestPrefix(manifest.Name),
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName:       manifest.Name,
					bdv1.LabelDeploymentSecretType: names.DeploymentSecretTypeManifestWithOps.String(),
					ejv1.LabelReferencedJobName:    fmt.Sprintf("instance-group-%s", manifest.Name),
				},
				Versioned: true,
			},
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
			UpdateOnConfigChange: true,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: eJobName,
					Labels: map[string]string{
						"delete": "pod",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:         VarInterpolationContainerName,
							Image:        GetOperatorDockerImage(),
							Args:         args,
							VolumeMounts: volumeMounts,
							Env: []corev1.EnvVar{
								{
									Name:  EnvBOSHManifestPath,
									Value: filepath.Join("/var/run/secrets/deployment/", bdm.DesiredManifestKeyName),
								},
								{
									Name:  EnvVariablesDir,
									Value: "/var/run/secrets/variables/",
								},
								{
									Name:  EnvOutputFilePath,
									Value: EnvOutputFilePathValue,
								},
							},
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
	return job, nil
}

// InstanceGroupManifestJob generates the job to create an instance group manifest
func (f *JobFactory) InstanceGroupManifestJob(manifest bdm.Manifest) (*ejv1.ExtendedJob, error) {
	containers := make([]corev1.Container, len(manifest.InstanceGroups))

	// ExtendedJob will always pick the latest version for versioned secrets
	desiredManifestName := names.DesiredManifestName(manifest.Name, "1")

	for idx, ig := range manifest.InstanceGroups {
		// One container per Instance Group
		// There will be one secret generated for each of these containers
		containers[idx] = f.gatheringContainer("instance-group", desiredManifestName, ig.Name)
	}

	eJobName := fmt.Sprintf("ig-%s", manifest.Name)
	return f.gatheringJob(eJobName, manifest, desiredManifestName, names.DeploymentSecretTypeInstanceGroupResolvedProperties, containers)
}

// BPMConfigsJob returns an extended job to calculate BPM information
func (f *JobFactory) BPMConfigsJob(manifest bdm.Manifest) (*ejv1.ExtendedJob, error) {
	containers := make([]corev1.Container, len(manifest.InstanceGroups))
	desiredManifestName := names.DesiredManifestName(manifest.Name, "1")

	for idx, ig := range manifest.InstanceGroups {
		// One container per Instance Group
		// There will be one secret generated for each of these containers
		containers[idx] = f.gatheringContainer("bpm-configs", desiredManifestName, ig.Name)
	}

	eJobName := fmt.Sprintf("bpm-%s", manifest.Name)
	return f.gatheringJob(eJobName, manifest, desiredManifestName, names.DeploymentSecretBpmInformation, containers)
}

func (f *JobFactory) gatheringContainer(cmd, desiredManifestName string, instanceGroupName string) corev1.Container {
	return corev1.Container{
		Name:  names.Sanitize(instanceGroupName),
		Image: GetOperatorDockerImage(),
		Args:  []string{"util", cmd},
		VolumeMounts: []corev1.VolumeMount{
			withOpsVolumeMount(desiredManifestName),
			releaseSourceVolumeMount(),
		},
		Env: []corev1.EnvVar{
			{
				Name:  EnvBOSHManifestPath,
				Value: filepath.Join("/var/run/secrets/deployment/", bdm.DesiredManifestKeyName),
			},
			{
				Name:  EnvCFONamespace,
				Value: f.Namespace,
			},
			{
				Name:  EnvBaseDir,
				Value: VolumeRenderingDataMountPath,
			},
			{
				Name:  EnvInstanceGroupName,
				Value: instanceGroupName,
			},
			{
				Name:  EnvOutputFilePath,
				Value: EnvOutputFilePathValue,
			},
		},
	}
}

func (f *JobFactory) gatheringJob(name string, manifest bdm.Manifest, desiredManifestName string, secretType names.DeploymentSecretType, containers []corev1.Container) (*ejv1.ExtendedJob, error) {
	outputSecretNamePrefix := names.CalculateIGSecretPrefix(secretType, manifest.Name)

	initContainers := []corev1.Container{}
	doneSpecCopyingReleases := map[string]bool{}
	for _, ig := range manifest.InstanceGroups {
		// Iterate through each Job to find all releases so we can copy all
		// sources to /var/vcap/instance-group
		for _, boshJob := range ig.Jobs {
			// If we've already generated an init container for this release, skip
			releaseName := boshJob.Release
			if _, ok := doneSpecCopyingReleases[releaseName]; ok {
				continue
			}
			doneSpecCopyingReleases[releaseName] = true

			// Get the docker image for the release
			releaseImage, err := (&manifest).GetReleaseImage(ig.Name, boshJob.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "Generation of gathering job failed for manifest %s", manifest.Name)
			}
			// Create an init container that copies sources
			// TODO: destination should also contain release name, to prevent overwrites
			initContainers = append(initContainers, jobSpecCopierContainer(releaseName, releaseImage, generateVolumeName(releaseSourceName)))
		}
	}

	// Construct the "BPM configs" or "data gathering" auto-errand eJob
	job := &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.Namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: manifest.Name,
			},
		},
		Spec: ejv1.ExtendedJobSpec{
			Output: &ejv1.Output{
				NamePrefix: outputSecretNamePrefix,
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName:       manifest.Name,
					bdv1.LabelDeploymentSecretType: secretType.String(),
				},
				Versioned: true,
			},
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
			UpdateOnConfigChange: true,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"delete": "pod",
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					// Init Container to copy contents
					InitContainers: initContainers,
					// Container to run data gathering
					Containers: containers,
					// Volumes for secrets
					Volumes: []corev1.Volume{
						*withOpsVolume(desiredManifestName),
						releaseSourceVolume(),
					},
				},
			},
		},
	}
	return job, nil
}
