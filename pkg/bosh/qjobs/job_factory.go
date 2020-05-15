package qjobs

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpmconverter"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers/quarksrestart"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/desiredmanifest"
	boshnames "code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/operatorimage"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
	"code.cloudfoundry.org/quarks-utils/pkg/versionedsecretstore"
)

const (
	// EnvCFONamespace is a key for the container Env used to lookup the
	// namespace CF operator is running in (CLI)
	EnvCFONamespace = "CF_OPERATOR_NAMESPACE"
	// EnvBaseDir is a key for the container Env used to lookup the base dir (CLI)
	EnvBaseDir = "BASE_DIR"
	// EnvVariablesDir is a key for the container Env used to lookup the variables dir (CLI)
	EnvVariablesDir = "VARIABLES_DIR"
	// EnvOutputFilePath is path where json output is to be redirected (CLI)
	EnvOutputFilePath = "OUTPUT_FILE_PATH"
	// EnvOutputFilePathValue is the value of filepath of JSON output dir
	EnvOutputFilePathValue = "/mnt/quarks"
	// outputFilename is the file name of the JSON output file, which quarks job will look for
	outputFilename = "output.json"
	// InstanceGroupOutputFilename i s the file name of the JSON output file, which quarks job will look for
	InstanceGroupOutputFilename = "ig.json"
	// BPMOutputFilename i s the file name of the JSON output file, which quarks job will look for
	BPMOutputFilename = "bpm.json"

	// PodNameEnvVar is the environment variable containing metadata.name used to render BOSH spec.id. (CLI)
	PodNameEnvVar = "POD_NAME"
)

// JobFactory is a concrete implementation of JobFactory
type JobFactory struct {
}

// NewJobFactory returns a concrete implementation of JobFactory
func NewJobFactory() *JobFactory {
	return &JobFactory{}
}

// VariableInterpolationJob returns an quarks job to create the desired manifest
// The desired manifest is a BOSH manifest with all variables interpolated.
func (f *JobFactory) VariableInterpolationJob(namespace string, deploymentName string, manifest bdm.Manifest) (*qjv1a1.QuarksJob, error) {
	args := []string{"util", "variable-interpolation"}

	// This is the source manifest, that still has the '((vars))'
	manifestSecretName := bdv1.DeploymentSecretTypeManifestWithOps.String()

	// Prepare Volumes and Volume mounts

	volumes := []corev1.Volume{*withOpsVolume(manifestSecretName)}
	volumeMounts := []corev1.VolumeMount{manifestVolumeMount(manifestSecretName)}

	// We need a volume and a mount for each input variable
	for _, variable := range manifest.Variables {
		varName := variable.Name
		varSecretName := boshnames.SecretVariableName(varName)

		volumes = append(volumes, variableVolume(varSecretName))
		volumeMounts = append(volumeMounts, variableVolumeMount(varSecretName, varName))
	}

	// If there are no variables, mount an empty dir for variables
	if len(manifest.Variables) == 0 {
		volumes = append(volumes, noVarsVolume())
		volumeMounts = append(volumeMounts, noVarsVolumeMount())
	}

	additionalLabels := map[string]string{
		bdv1.LabelEntanglementKey: "true",
	}
	additionalAnnotations := map[string]string{
		quarksrestart.AnnotationRestartOnUpdate: "true",
	}

	// Construct the var interpolation auto-errand qJob
	qJob := &qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dm",
			Namespace: namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deploymentName,
			},
		},
		Spec: qjv1a1.QuarksJobSpec{
			Output: &qjv1a1.Output{
				OutputMap: qjv1a1.OutputMap{
					desiredmanifest.Name: qjv1a1.NewFileToSecret(outputFilename, desiredmanifest.Name, true, additionalAnnotations, additionalLabels),
				},
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName:       deploymentName,
					bdv1.LabelDeploymentSecretType: bdv1.DeploymentSecretTypeDesiredManifest.String(),
					bdv1.LabelReferencedJobName:    fmt.Sprintf("instance-group-%s", deploymentName),
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
							Name: "dm",
							Labels: map[string]string{
								"delete":                 "pod",
								bdv1.LabelDeploymentName: deploymentName,
							},
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:            desiredmanifest.Name,
									Image:           operatorimage.GetOperatorDockerImage(),
									ImagePullPolicy: operatorimage.GetOperatorImagePullPolicy(),
									Args:            args,
									VolumeMounts:    volumeMounts,
									Env: []corev1.EnvVar{
										{
											Name:  bpmconverter.EnvBOSHManifestPath,
											Value: filepath.Join("/var/run/secrets/deployment/", bdm.DesiredManifestKeyName),
										},
										{
											Name:  EnvVariablesDir,
											Value: "/var/run/secrets/variables/",
										},
										{
											Name:  EnvOutputFilePath,
											Value: filepath.Join(EnvOutputFilePathValue, outputFilename),
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
func (f *JobFactory) InstanceGroupManifestJob(namespace string, deploymentName string, manifest bdm.Manifest, linkInfos converter.LinkInfos, initialRollout bool) (*qjv1a1.QuarksJob, error) {
	dmName := desiredManifestName()
	ct := containerTemplate{
		deploymentName: deploymentName,
		manifestName:   dmName,
		cmd:            "instance-group",
		namespace:      namespace,
		initialRollout: initialRollout,
	}

	containers := []corev1.Container{}
	linkOutputs := map[string]string{}
	for _, ig := range manifest.InstanceGroups {
		if ig.Instances != 0 {
			// Additional secret for BOSH links per instance group
			containerName := names.Sanitize(ig.Name)
			linkOutputs[containerName] = boshnames.QuarksLinkSecretName()

			// One container per instance group
			containers = append(containers, ct.newUtilContainer(ig.Name, linkInfos.VolumeMounts()))
		}
	}

	qJob, err := f.releaseImageQJob(namespace, deploymentName, dmName, manifest, containers, linkInfos.Volumes())
	if err != nil {
		return nil, err
	}

	// add the BOSH link secret to the output list of each container
	for container, secret := range linkOutputs {
		qJob.Spec.Output.OutputMap[container]["provides.json"] = qjv1a1.SecretOptions{
			Name:              secret,
			PersistenceMethod: qjv1a1.PersistUsingFanOut,
			AdditionalSecretLabels: map[string]string{
				bdv1.LabelEntanglementKey: "true",
			},
			AdditionalSecretAnnotations: map[string]string{
				quarksrestart.AnnotationRestartOnUpdate: "true",
			},
		}
	}

	return qJob, nil
}

// desiredManifestName returns the sanitized, versioned name of the manifest.
// QuarksJob will always pick the latest version for versioned secrets
func desiredManifestName() string {
	return versionedsecretstore.VersionedName(desiredmanifest.Name, 1)
}

type containerTemplate struct {
	deploymentName string
	manifestName   string
	cmd            string
	namespace      string
	initialRollout bool
}

func (ct *containerTemplate) newUtilContainer(instanceGroupName string, linkVolumeMounts []corev1.VolumeMount) corev1.Container {
	return corev1.Container{
		Name:            names.Sanitize(instanceGroupName),
		Image:           operatorimage.GetOperatorDockerImage(),
		ImagePullPolicy: operatorimage.GetOperatorImagePullPolicy(),
		Args:            []string{"util", ct.cmd, "--initial-rollout", strconv.FormatBool(ct.initialRollout)},
		VolumeMounts: append(linkVolumeMounts, []corev1.VolumeMount{
			manifestVolumeMount(ct.manifestName),
			releaseSourceVolumeMount(),
		}...),
		Env: []corev1.EnvVar{
			{
				Name:  bpmconverter.EnvDeploymentName,
				Value: ct.deploymentName,
			},
			{
				Name:  bpmconverter.EnvBOSHManifestPath,
				Value: filepath.Join("/var/run/secrets/deployment/", bdm.DesiredManifestKeyName),
			},
			{
				Name:  EnvCFONamespace,
				Value: ct.namespace,
			},
			{
				Name:  EnvBaseDir,
				Value: bpmconverter.VolumeRenderingDataMountPath,
			},
			{
				Name:  bpmconverter.EnvInstanceGroupName,
				Value: instanceGroupName,
			},
			{
				Name:  EnvOutputFilePath,
				Value: EnvOutputFilePathValue,
			},
			{
				Name:  qjv1a1.RemoteIDKey,
				Value: instanceGroupName,
			},
		},
	}
}

// releaseImageQJob collects outputs, like bpm, links or ig manifests, from the BOSH release images
func (f *JobFactory) releaseImageQJob(namespace string, deploymentName string, dmName string, manifest bdm.Manifest, containers []corev1.Container, linkVolumes []corev1.Volume) (*qjv1a1.QuarksJob, error) {
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
				return nil, errors.Wrapf(err, "Generation of gathering job '%s' failed for instance group '%s'", boshJob.Name, ig.Name)
			}
			// Create an init container that copies sources
			// TODO: destination should also contain release name, to prevent overwrites
			initContainers = append(initContainers, bpmconverter.JobSpecCopierContainer(releaseName, releaseImage, names.VolumeName(releaseSourceName)))
		}
	}

	outputMap := qjv1a1.OutputMap{}
	igPrefix := bdv1.DeploymentSecretTypeInstanceGroupResolvedProperties.Prefix()
	bpmPrefix := bdv1.DeploymentSecretBPMInformation.Prefix()
	for _, container := range containers {
		outputMap[container.Name] = qjv1a1.FilesToSecrets{
			InstanceGroupOutputFilename: qjv1a1.SecretOptions{
				// the same as names.InstanceGroupSecretName(container.Name, "")
				Name: igPrefix + container.Name,
				AdditionalSecretLabels: map[string]string{
					bdv1.LabelEntanglementKey:      "true",
					bdv1.LabelDeploymentSecretType: bdv1.DeploymentSecretTypeInstanceGroupResolvedProperties.String(),
				},
				AdditionalSecretAnnotations: map[string]string{
					quarksrestart.AnnotationRestartOnUpdate: "true",
				},
				Versioned: true,
			},
			BPMOutputFilename: qjv1a1.SecretOptions{
				Name: bpmPrefix + container.Name,
				AdditionalSecretLabels: map[string]string{
					bdv1.LabelEntanglementKey:      "true",
					bdv1.LabelDeploymentSecretType: bdv1.DeploymentSecretBPMInformation.String(),
				},
				AdditionalSecretAnnotations: map[string]string{
					quarksrestart.AnnotationRestartOnUpdate: "true",
				},
				Versioned: true,
			},
		}
	}

	// Construct the "BPM configs" or "data gathering" auto-errand qJob
	qJob := &qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ig",
			Namespace: namespace,
			Labels: map[string]string{
				bdv1.LabelDeploymentName: deploymentName,
			},
		},
		Spec: qjv1a1.QuarksJobSpec{
			Output: &qjv1a1.Output{
				OutputMap: outputMap,
				SecretLabels: map[string]string{
					bdv1.LabelDeploymentName: deploymentName,
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
							Name: "ig",
							Labels: map[string]string{
								"delete":                 "pod",
								bdv1.LabelDeploymentName: "deploymentName",
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
								*withOpsVolume(dmName),
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
