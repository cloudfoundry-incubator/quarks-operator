package converter

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

const (
	// EnvInstanceGroupName is a key for the container Env identifying the
	// instance group that container is started for (CLI)
	EnvInstanceGroupName = "INSTANCE_GROUP_NAME"
	// EnvBOSHManifestPath is a key for the container Env pointing to the BOSH manifest (CLI)
	EnvBOSHManifestPath = "BOSH_MANIFEST_PATH"
	// EnvCFONamespace is a key for the container Env used to lookup the
	// namespace CF operator is running in (CLI)
	EnvCFONamespace = "CF_OPERATOR_NAMESPACE"
	// EnvBaseDir is a key for the container Env used to lookup the base dir (CLI)
	EnvBaseDir = "BASE_DIR"
	// EnvVariablesDir is a key for the container Env used to lookup the variables dir (CLI)
	EnvVariablesDir = "VARIABLES_DIR"
	// EnvOutputFilePath is path where json output is to be redirected (CLI)
	EnvOutputFilePath = "OUTPUT_FILE_PATH"
	// EnvOutputFilePathValue is the value of filepath of JSON output file
	EnvOutputFilePathValue = "/mnt/quarks/output.json"
	// outputFilename is the file name of the JSON output file, which quarks job will look for
	outputFilename = "output.json"

	// VarInterpolationContainerName is the name of the container that
	// performs variable interpolation for a manifest. It's also part of
	// the output secret's name
	VarInterpolationContainerName = "desired-manifest"
	// PodNameEnvVar is the environment variable containing metadata.name used to render BOSH spec.id. (CLI)
	PodNameEnvVar = "POD_NAME"
	// PodIPEnvVar is the environment variable containing status.podIP used to render BOSH spec.ip. (CLI)
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

// VariableInterpolationJob returns an quarks job to interpolate variables
func (f *JobFactory) VariableInterpolationJob(manifest bdm.Manifest) (*qjv1a1.QuarksJob, error) {
	args := []string{"util", "variable-interpolation"}

	// This is the source manifest, that still has the '((vars))'
	manifestSecretName := names.DeploymentSecretName(names.DeploymentSecretTypeManifestWithOps, manifest.Name, "")

	// Prepare Volumes and Volume mounts

	volumes := []corev1.Volume{*withOpsVolume(manifestSecretName)}
	volumeMounts := []corev1.VolumeMount{withOpsVolumeMount(manifestSecretName)}

	// We need a volume and a mount for each input variable
	for _, variable := range manifest.Variables {
		varName := variable.Name
		varSecretName := names.DeploymentSecretName(names.DeploymentSecretTypeVariable, manifest.Name, varName)

		volumes = append(volumes, variableVolume(varSecretName))
		volumeMounts = append(volumeMounts, variableVolumeMount(varSecretName, varName))
	}

	// If there are no variables, mount an empty dir for variables
	if len(manifest.Variables) == 0 {
		volumes = append(volumes, noVarsVolume())
		volumeMounts = append(volumeMounts, noVarsVolumeMount())
	}

	qJobName := fmt.Sprintf("dm-%s", manifest.Name)
	secretName := names.DesiredManifestPrefix(manifest.Name) + VarInterpolationContainerName

	// Construct the var interpolation auto-errand qJob
	qJob := &qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      qJobName,
			Namespace: f.Namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: manifest.Name,
			},
		},
		Spec: qjv1a1.QuarksJobSpec{
			Output: &qjv1a1.Output{
				OutputMap: qjv1a1.OutputMap{
					VarInterpolationContainerName: qjv1a1.NewFileToSecret("output.json", secretName, true),
				},
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName:       manifest.Name,
					bdv1.LabelDeploymentSecretType: names.DeploymentSecretTypeManifestWithOps.String(),
					bdm.LabelReferencedJobName:     fmt.Sprintf("instance-group-%s", manifest.Name),
				},
			},
			Trigger: qjv1a1.Trigger{
				Strategy: qjv1a1.TriggerOnce,
			},
			UpdateOnConfigChange: true,
			Template: batchv1b1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: qJobName,
							Labels: map[string]string{
								"delete": "pod",
							},
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:            VarInterpolationContainerName,
									Image:           GetOperatorDockerImage(),
									ImagePullPolicy: GetOperatorImagePullPolicy(),
									Args:            args,
									VolumeMounts:    volumeMounts,
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
			},
		},
	}
	return qJob, nil
}

// InstanceGroupManifestJob generates the job to create an instance group manifest
func (f *JobFactory) InstanceGroupManifestJob(manifest bdm.Manifest, linkInfos LinkInfos, initialRollout bool) (*qjv1a1.QuarksJob, error) {
	containers := []corev1.Container{}
	ct := containerTemplate{
		manifestName:   desiredManifestName(manifest.Name),
		cmd:            "instance-group",
		namespace:      f.Namespace,
		initialRollout: initialRollout,
	}

	linkOutputs := map[string]string{}

	for _, ig := range manifest.InstanceGroups {
		if ig.Instances != 0 {
			// Additional secret for BOSH links per instance group
			containerName := names.Sanitize(ig.Name)
			linkOutputs[containerName] = names.EntanglementSecretName(manifest.Name, ig.Name)

			// One container per instance group
			containers = append(containers, ct.newUtilContainer(ig.Name, linkInfos.VolumeMounts()))
		}
	}

	qJobName := fmt.Sprintf("ig-%s", manifest.Name)

	qJob, err := f.releaseImageQJob(qJobName, manifest, names.DeploymentSecretTypeInstanceGroupResolvedProperties, containers, linkInfos.Volumes())
	if err != nil {
		return nil, err
	}

	// add BOSH link secret to the output list
	for container, secret := range linkOutputs {
		qJob.Spec.Output.OutputMap[container]["provides.json"] = qjv1a1.SecretOptions{
			Name: secret,
		}
	}
	return qJob, nil
}

// BPMConfigsJob returns an quarks job to calculate BPM information
func (f *JobFactory) BPMConfigsJob(manifest bdm.Manifest, linkInfos LinkInfos, initialRollout bool) (*qjv1a1.QuarksJob, error) {
	containers := []corev1.Container{}
	ct := containerTemplate{
		manifestName:   desiredManifestName(manifest.Name),
		cmd:            "bpm-configs",
		namespace:      f.Namespace,
		initialRollout: initialRollout,
	}

	for _, ig := range manifest.InstanceGroups {
		if ig.Instances != 0 {
			// One container per instance group
			// There will be one BPM secret generated for each of these containers
			container := ct.newUtilContainer(ig.Name, linkInfos.VolumeMounts())

			env := corev1.EnvVar{Name: qjv1a1.RemoteIDKey, Value: ig.Name}
			container.Env = append(container.Env, env)
			containers = append(containers, container)
		}
	}

	qJobName := fmt.Sprintf("bpm-%s", manifest.Name)
	return f.releaseImageQJob(qJobName, manifest, names.DeploymentSecretBpmInformation, containers, linkInfos.Volumes())
}

// desiredManifestName returns the sanitized, versioned name of the manifest.
// QuarksJob will always pick the latest version for versioned secrets
func desiredManifestName(name string) string {
	return names.DesiredManifestName(name, "1")
}

type containerTemplate struct {
	manifestName   string
	cmd            string
	namespace      string
	initialRollout bool
}

func (ct *containerTemplate) newUtilContainer(instanceGroupName string, linkVolumeMounts []corev1.VolumeMount) corev1.Container {
	return corev1.Container{
		Name:            names.Sanitize(instanceGroupName),
		Image:           GetOperatorDockerImage(),
		ImagePullPolicy: GetOperatorImagePullPolicy(),
		Args:            []string{"util", ct.cmd, "--initial-rollout", strconv.FormatBool(ct.initialRollout)},
		VolumeMounts: append(linkVolumeMounts, []corev1.VolumeMount{
			withOpsVolumeMount(ct.manifestName),
			releaseSourceVolumeMount(),
		}...),
		Env: []corev1.EnvVar{
			{
				Name:  EnvBOSHManifestPath,
				Value: filepath.Join("/var/run/secrets/deployment/", bdm.DesiredManifestKeyName),
			},
			{
				Name:  EnvCFONamespace,
				Value: ct.namespace,
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

// releaseImageQJob collects outputs, like bpm, links or ig manifests, from the BOSH release images
func (f *JobFactory) releaseImageQJob(name string, manifest bdm.Manifest, secretType names.DeploymentSecretType, containers []corev1.Container, linkVolumes []corev1.Volume) (*qjv1a1.QuarksJob, error) {
	initContainers := []corev1.Container{}
	doneSpecCopyingReleases := map[string]bool{}
	for _, ig := range manifest.InstanceGroups {
		if ig.Instances == 0 {
			continue
		}
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

	outputMap := qjv1a1.OutputMap{}
	outputSecretNamePrefix := names.DeploymentSecretPrefix(secretType, manifest.Name)
	for _, container := range containers {
		outputMap[container.Name] = qjv1a1.NewFileToSecret(
			outputFilename,
			// the same as names.InstaceGroupSecretName(secretType, manifestName, container.Name, "")
			outputSecretNamePrefix+container.Name,
			true,
		)
	}

	// Construct the "BPM configs" or "data gathering" auto-errand qJob
	qJob := &qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: f.Namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: manifest.Name,
			},
		},
		Spec: qjv1a1.QuarksJobSpec{
			Output: &qjv1a1.Output{
				OutputMap: outputMap,
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName:       manifest.Name,
					bdv1.LabelDeploymentSecretType: secretType.String(),
				},
			},
			Trigger: qjv1a1.Trigger{
				Strategy: qjv1a1.TriggerOnce,
			},
			UpdateOnConfigChange: true,
			Template: batchv1b1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
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
							Volumes: append(linkVolumes, []corev1.Volume{
								*withOpsVolume(desiredManifestName(manifest.Name)),
								releaseSourceVolume(),
							}...),
						},
					},
				},
			},
		},
	}
	return qJob, nil
}
