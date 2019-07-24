package converter

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
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
	// VarInterpolationContainerName is the name of the container that
	// performs variable interpolation for a manifest. It's also part of
	// the output secret's name
	VarInterpolationContainerName = "desired-manifest"
	// PodIPEnvVar is the environment variable containing status.podIP used to render BOSH spec.IP.
	PodIPEnvVar = "POD_IP"
)

// JobFactory creates Jobs for a given manifest
type JobFactory struct {
	Manifest            bdm.Manifest
	Namespace           string
	desiredManifestName string
}

// NewJobFactory returns a new JobFactory
func NewJobFactory(manifest bdm.Manifest, namespace string) *JobFactory {
	return &JobFactory{
		Manifest:  manifest,
		Namespace: namespace,
		// ExtendedJob will always pick the latest version for versioned secrets
		desiredManifestName: names.DesiredManifestName(manifest.Name, "1"),
	}
}

// VariableInterpolationJob returns an extended job to interpolate variables
func (f *JobFactory) VariableInterpolationJob() (*ejv1.ExtendedJob, error) {
	args := []string{"util", "variable-interpolation"}

	// This is the source manifest, that still has the '((vars))'
	manifestSecretName := names.CalculateSecretName(names.DeploymentSecretTypeManifestWithOps, f.Manifest.Name, "")

	// Prepare Volumes and Volume mounts

	// This is a volume for the "not interpolated" manifest,
	// that has the ops files applied, but still contains '((vars))'
	volumes := []corev1.Volume{
		{
			Name: generateVolumeName(manifestSecretName),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: manifestSecretName,
				},
			},
		},
	}
	// Volume mount for the manifest
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      generateVolumeName(manifestSecretName),
			MountPath: "/var/run/secrets/deployment/",
			ReadOnly:  true,
		},
	}

	// We need a volume and a mount for each input variable
	for _, variable := range f.Manifest.Variables {
		varName := variable.Name
		varSecretName := names.CalculateSecretName(names.DeploymentSecretTypeVariable, f.Manifest.Name, varName)

		// The volume definition
		vol := corev1.Volume{
			Name: generateVolumeName(varSecretName),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: varSecretName,
				},
			},
		}
		volumes = append(volumes, vol)

		// And the volume mount
		volMount := corev1.VolumeMount{
			Name:      generateVolumeName(varSecretName),
			MountPath: "/var/run/secrets/variables/" + varName,
			ReadOnly:  true,
		}
		volumeMounts = append(volumeMounts, volMount)
	}

	// If there are no variables, mount an empty dir for variables
	if len(f.Manifest.Variables) == 0 {
		// The volume definition
		vol := corev1.Volume{
			Name: generateVolumeName("no-vars"),
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
		volumes = append(volumes, vol)

		// And the volume mount
		volMount := corev1.VolumeMount{
			Name:      generateVolumeName("no-vars"),
			MountPath: "/var/run/secrets/variables/",
			ReadOnly:  true,
		}
		volumeMounts = append(volumeMounts, volMount)
	}

	// Calculate the signature of the manifest, to label things
	manifestSignature, err := f.Manifest.SHA1()
	if err != nil {
		return nil, errors.Wrapf(err, "Generation of Variable interpolation job failed.")
	}

	eJobName := fmt.Sprintf("dm-%s", f.Manifest.Name)

	// Construct the var interpolation auto-errand eJob
	job := &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      eJobName,
			Namespace: f.Namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: f.Manifest.Name,
			},
		},
		Spec: ejv1.ExtendedJobSpec{
			Output: &ejv1.Output{
				NamePrefix: names.DesiredManifestPrefix(f.Manifest.Name),
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName:       f.Manifest.Name,
					bdv1.LabelManifestSHA1:         manifestSignature,
					bdv1.LabelDeploymentSecretType: names.DeploymentSecretTypeManifestWithOps.String(),
					ejv1.LabelReferencedJobName:    fmt.Sprintf("instance-group-%s", f.Manifest.Name),
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
func (f *JobFactory) InstanceGroupManifestJob() (*ejv1.ExtendedJob, error) {
	containers := make([]corev1.Container, len(f.Manifest.InstanceGroups))

	for idx, ig := range f.Manifest.InstanceGroups {
		// One container per Instance Group
		// There will be one secret generated for each of these containers
		containers[idx] = f.gatheringContainer("instance-group", ig.Name)
	}

	eJobName := fmt.Sprintf("ig-%s", f.Manifest.Name)
	return f.gatheringJob(eJobName, names.DeploymentSecretTypeInstanceGroupResolvedProperties, containers)
}

// BPMConfigsJob returns an extended job to calculate BPM information
func (f *JobFactory) BPMConfigsJob() (*ejv1.ExtendedJob, error) {
	containers := make([]corev1.Container, len(f.Manifest.InstanceGroups))

	for idx, ig := range f.Manifest.InstanceGroups {
		// One container per Instance Group
		// There will be one secret generated for each of these containers
		containers[idx] = f.gatheringContainer("bpm-configs", ig.Name)
	}

	eJobName := fmt.Sprintf("bpm-%s", f.Manifest.Name)
	return f.gatheringJob(eJobName, names.DeploymentSecretBpmInformation, containers)
}

func (f *JobFactory) gatheringContainer(cmd, instanceGroupName string) corev1.Container {
	return corev1.Container{
		Name:  names.Sanitize(instanceGroupName),
		Image: GetOperatorDockerImage(),
		Args:  []string{"util", cmd},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      generateVolumeName(f.desiredManifestName),
				MountPath: "/var/run/secrets/deployment/",
				ReadOnly:  true,
			},
			{
				Name:      generateVolumeName("instance-group"),
				MountPath: VolumeRenderingDataMountPath,
			},
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
		},
	}
}

func (f *JobFactory) gatheringJob(name string, secretType names.DeploymentSecretType, containers []corev1.Container) (*ejv1.ExtendedJob, error) {
	outputSecretNamePrefix := names.CalculateIGSecretPrefix(secretType, f.Manifest.Name)

	initContainers := []corev1.Container{}
	doneSpecCopyingReleases := map[string]bool{}
	for _, ig := range f.Manifest.InstanceGroups {
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
			releaseImage, err := (&f.Manifest).GetReleaseImage(ig.Name, boshJob.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "Generation of gathering job failed for manifest %s", f.Manifest.Name)
			}
			// Create an init container that copies sources
			// TODO: destination should also contain release name, to prevent overwrites
			initContainers = append(initContainers, jobSpecCopierContainer(releaseName, releaseImage, generateVolumeName("instance-group")))
		}
	}

	// Construct the "BPM configs" or "data gathering" auto-errand eJob
	job := &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.Namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: f.Manifest.Name,
			},
		},
		Spec: ejv1.ExtendedJobSpec{
			Output: &ejv1.Output{
				NamePrefix: outputSecretNamePrefix,
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName:       f.Manifest.Name,
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
						{
							Name: generateVolumeName(f.desiredManifestName),
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: f.desiredManifestName,
								},
							},
						},
						{
							Name: generateVolumeName("instance-group"),
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
	return job, nil
}
