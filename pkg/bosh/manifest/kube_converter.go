package manifest

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

const (
	// VarInterpolationContainerName is the name of the container that performs
	// variable interpolation for a manifest
	VarInterpolationContainerName = "interpolation"
	// DesiredManifestKeyName is the name of the key in desired manifest secret
	DesiredManifestKeyName = "manifest.yaml"
)

var (
	// LabelDeploymentName is the name of a label for the deployment name
	LabelDeploymentName = fmt.Sprintf("%s/deployment-name", apis.GroupName)
	// LabelInstanceGroupName is the name of a label for an instance group name
	LabelInstanceGroupName = fmt.Sprintf("%s/instance-group-name", apis.GroupName)
)

// KubeConfig represents a Manifest in kube resources
type KubeConfig struct {
	namespace                string
	releaseImageProvider     releaseImageProvider
	manifestName             string
	Variables                []esv1.ExtendedSecret
	InstanceGroups           []essv1.ExtendedStatefulSet
	Errands                  []ejv1.ExtendedJob
	Services                 []corev1.Service
	VariableInterpolationJob *ejv1.ExtendedJob
	DataGatheringJob         *ejv1.ExtendedJob
	BPMConfigsJob            *ejv1.ExtendedJob
}

type releaseImageProvider interface {
	GetReleaseImage(instanceGroupName, jobName string) (string, error)
}

// NewKubeConfig converts a Manifest into kube resources
func NewKubeConfig(namespace string, rip releaseImageProvider) *KubeConfig {
	return &KubeConfig{
		namespace:            namespace,
		releaseImageProvider: rip,
	}
}

func (kc *KubeConfig) Convert(m Manifest) error {
	kc.manifestName = m.Name

	convertedExtSts, convertedSvcs, err := kc.convertToExtendedStsAndServices(m.InstanceGroups)
	if err != nil {
		return err
	}

	convertedEJob, err := kc.convertToExtendedJob(m.InstanceGroups)
	if err != nil {
		return err
	}

	jobFactory := NewJobFactory(m, kc.namespace)
	dataGatheringJob, err := jobFactory.DataGatheringJob()
	if err != nil {
		return err
	}

	bpmConfigsJob, err := jobFactory.BPMConfigsJob()
	if err != nil {
		return err
	}

	varInterpolationJob, err := jobFactory.VariableInterpolationJob()
	if err != nil {
		return err
	}

	kc.Variables = kc.convertVariables(m.Variables)
	kc.InstanceGroups = convertedExtSts
	kc.Services = convertedSvcs
	kc.Errands = convertedEJob
	kc.VariableInterpolationJob = varInterpolationJob
	kc.DataGatheringJob = dataGatheringJob
	kc.BPMConfigsJob = bpmConfigsJob

	return nil
}

// ApplyBPMInfo uses BOSH Process Manager information to update container information like entrypoint, env vars, etc.
func (kc *KubeConfig) ApplyBPMInfo(allBPMConfigs map[string]bpm.Configs) error {

	applyBPMOnContainer := func(igName string, container *corev1.Container) error {
		boshJobName := container.Name

		igBPMConfigs, ok := allBPMConfigs[igName]
		if !ok {
			return errors.Errorf("couldn't find instance group '%s' in bpm configs set", igName)
		}

		bpmConfig, ok := igBPMConfigs[boshJobName]
		if !ok {
			return errors.Errorf("failed to lookup bpm config for bosh job '%s' in bpm configs for instance group '%s'", boshJobName, igName)
		}

		// TODO: handle multi-process BPM?
		// TODO: complete implementation - BPM information could be top-level only

		if len(bpmConfig.Processes) < 1 {
			return errors.New("bpm info has no processes")
		}
		process := bpmConfig.Processes[0]

		container.Command = []string{process.Executable}
		container.Args = process.Args
		for name, value := range process.Env {
			container.Env = append(container.Env, corev1.EnvVar{Name: name, Value: value})
		}
		container.WorkingDir = process.Workdir

		return nil
	}

	for idx := range kc.InstanceGroups {
		igSts := &(kc.InstanceGroups[idx])
		igName := igSts.Labels[LabelInstanceGroupName]

		// Go through each container
		for idx := range igSts.Spec.Template.Spec.Template.Spec.Containers {
			container := &(igSts.Spec.Template.Spec.Template.Spec.Containers[idx])
			err := applyBPMOnContainer(igName, container)

			if err != nil {
				return errors.Wrapf(err, "failed to apply bpm information on bosh job '%s', instance group '%s'", container.Name, igName)
			}
		}
	}

	for idx := range kc.Errands {
		igJob := &(kc.Errands[idx])
		igName := igJob.Labels[LabelInstanceGroupName]

		for idx := range igJob.Spec.Template.Spec.Containers {
			container := &(igJob.Spec.Template.Spec.Containers[idx])
			err := applyBPMOnContainer(igName, container)

			if err != nil {
				return errors.Wrapf(err, "failed to apply bpm information on bosh job '%s', instance group '%s'", container.Name, igName)
			}
		}
	}
	return nil
}

// jobsToInitContainers creates a list of Containers for corev1.PodSpec InitContainers field
func (kc *KubeConfig) jobsToInitContainers(igName string, jobs []Job, namespace string) ([]corev1.Container, error) {
	initContainers := []corev1.Container{}

	// one init container for each release, for copying specs
	doneReleases := map[string]bool{}
	for _, job := range jobs {
		if _, ok := doneReleases[job.Release]; ok {
			continue
		}

		doneReleases[job.Release] = true
		releaseImage, err := kc.releaseImageProvider.GetReleaseImage(igName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}
		initContainers = append(initContainers, jobSpecCopierContainer(job.Release, releaseImage, "rendering-data"))

	}

	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		kc.manifestName,
		igName,
		true,
	)

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "rendering-data",
			MountPath: "/var/vcap/all-releases",
		},
		{
			Name:      "jobs-dir",
			MountPath: "/var/vcap/jobs",
		},
		{
			Name:      generateVolumeName(resolvedPropertiesSecretName),
			MountPath: fmt.Sprintf("/var/run/secrets/resolved-properties/%s", igName),
			ReadOnly:  true,
		},
	}

	initContainers = append(initContainers, corev1.Container{
		Name:         fmt.Sprintf("renderer-%s", igName),
		Image:        GetOperatorDockerImage(),
		VolumeMounts: volumeMounts,
		Env: []corev1.EnvVar{
			{
				Name:  "INSTANCE_GROUP_NAME",
				Value: igName,
			},
			{
				Name:  "BOSH_MANIFEST_PATH",
				Value: fmt.Sprintf("/var/run/secrets/resolved-properties/%s/properties.yaml", igName),
			},
			{
				Name:  "JOBS_DIR",
				Value: "/var/vcap/all-releases",
			},
		},
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", `cf-operator util template-render`},
	})

	return initContainers, nil
}

// jobsToContainers creates a list of Containers for corev1.PodSpec Containers field
func (kc *KubeConfig) jobsToContainers(igName string, jobs []Job, namespace string) ([]corev1.Container, error) {
	var jobsToContainerPods []corev1.Container

	if len(jobs) == 0 {
		return nil, fmt.Errorf("instance group %s has no jobs defined", igName)
	}

	for _, job := range jobs {
		jobImage, err := kc.releaseImageProvider.GetReleaseImage(igName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}
		jobsToContainerPods = append(jobsToContainerPods, corev1.Container{
			Name:  fmt.Sprintf(job.Name),
			Image: jobImage,
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "rendering-data",
					MountPath: "/var/vcap/all-releases",
				},
				{
					Name:      "jobs-dir",
					MountPath: "/var/vcap/jobs",
				},
			},
		})
	}
	return jobsToContainerPods, nil
}

// serviceToExtendedSts will generate an ExtendedStatefulSet
func (kc *KubeConfig) serviceToExtendedSts(ig *InstanceGroup, namespace string) (essv1.ExtendedStatefulSet, error) {
	igName := ig.Name

	// TODO this is not one to one
	listOfContainers, err := kc.jobsToContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	listOfInitContainers, err := kc.jobsToInitContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	_, interpolatedManifestSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeManifestAndVars,
		kc.manifestName,
		VarInterpolationContainerName,
		true,
	)
	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		kc.manifestName,
		ig.Name,
		true,
	)

	volumes := []corev1.Volume{
		{
			Name:         "rendering-data",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name:         "jobs-dir",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name: generateVolumeName(interpolatedManifestSecretName),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: interpolatedManifestSecretName,
				},
			},
		},
		{
			Name: generateVolumeName(resolvedPropertiesSecretName),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: resolvedPropertiesSecretName,
				},
			},
		},
	}

	extSts := essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", kc.manifestName, igName),
			Namespace: namespace,
			Labels: map[string]string{
				LabelInstanceGroupName: igName,
			},
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			UpdateOnConfigChange: true,
			Template: v1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: igName,
				},
				Spec: v1beta2.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(ig.Instances); return &i }(),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							bdv1.LabelDeploymentName: kc.manifestName,
							LabelInstanceGroupName:   igName,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: igName,
							Labels: map[string]string{
								bdv1.LabelDeploymentName: kc.manifestName,
								LabelInstanceGroupName:   igName,
							},
						},
						Spec: corev1.PodSpec{
							Volumes:        volumes,
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

// serviceToKubeServices will generate Services which expose ports for InstanceGroup's jobs
func (kc *KubeConfig) serviceToKubeServices(ig *InstanceGroup, eSts *essv1.ExtendedStatefulSet, namespace string) ([]corev1.Service, error) {
	var services []corev1.Service
	igName := ig.Name

	// Collect ports to be exposed for each job
	ports := []corev1.ServicePort{}
	for _, job := range ig.Jobs {
		for _, port := range job.Properties.BOSHContainerization.Ports {
			ports = append(ports, corev1.ServicePort{
				Name:     port.Name,
				Protocol: corev1.Protocol(port.Protocol),
				Port:     int32(port.Internal),
			})
		}

	}

	if len(ports) == 0 {
		return services, nil
	}

	for i := 0; i < ig.Instances; i++ {
		if len(ig.AZs) == 0 {
			services = append(services, corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.ServiceName(kc.manifestName, igName, len(services)),
					Namespace: namespace,
					Labels: map[string]string{
						LabelInstanceGroupName: igName,
						essv1.LabelAZIndex:     strconv.Itoa(0),
						essv1.LabelPodOrdinal:  strconv.Itoa(i),
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: ports,
					Selector: map[string]string{
						LabelInstanceGroupName: igName,
						essv1.LabelAZIndex:     strconv.Itoa(0),
						essv1.LabelPodOrdinal:  strconv.Itoa(i),
					},
				},
			})
		}
		for azIndex := range ig.AZs {
			services = append(services, corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      names.ServiceName(kc.manifestName, igName, len(services)),
					Namespace: namespace,
					Labels: map[string]string{
						LabelInstanceGroupName: igName,
						essv1.LabelAZIndex:     strconv.Itoa(azIndex),
						essv1.LabelPodOrdinal:  strconv.Itoa(i),
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: ports,
					Selector: map[string]string{
						LabelInstanceGroupName: igName,
						essv1.LabelAZIndex:     strconv.Itoa(azIndex),
						essv1.LabelPodOrdinal:  strconv.Itoa(i),
					},
				},
			})
		}
	}

	headlessService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.ServiceName(kc.manifestName, igName, -1),
			Namespace: namespace,
			Labels: map[string]string{
				LabelInstanceGroupName: igName,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
			Selector: map[string]string{
				LabelInstanceGroupName: igName,
			},
			ClusterIP: "None",
		},
	}

	services = append(services, headlessService)

	// Set headlessService to govern StatefulSet
	eSts.Spec.Template.Spec.ServiceName = names.ServiceName(kc.manifestName, igName, -1)

	return services, nil
}

// convertToExtendedStsAndServices will convert instance_groups whose lifecycle
// is service, to ExtendedStatefulSets and their Services
func (kc *KubeConfig) convertToExtendedStsAndServices(instanceGroups []*InstanceGroup) ([]essv1.ExtendedStatefulSet, []corev1.Service, error) {
	extStsList := []essv1.ExtendedStatefulSet{}
	svcList := []corev1.Service{}

	for _, ig := range instanceGroups {
		if ig.LifeCycle == "service" || ig.LifeCycle == "" {
			convertedExtStatefulSet, err := kc.serviceToExtendedSts(ig, kc.namespace)
			if err != nil {
				return []essv1.ExtendedStatefulSet{}, []corev1.Service{}, err
			}

			services, err := kc.serviceToKubeServices(ig, &convertedExtStatefulSet, kc.namespace)
			if err != nil {
				return []essv1.ExtendedStatefulSet{}, []corev1.Service{}, err
			}
			if len(services) != 0 {
				svcList = append(svcList, services...)
			}

			extStsList = append(extStsList, convertedExtStatefulSet)
		}
	}

	return extStsList, svcList, nil
}

// errandToExtendedJob will generate an ExtendedJob
func (kc *KubeConfig) errandToExtendedJob(ig *InstanceGroup, namespace string) (ejv1.ExtendedJob, error) {
	igName := ig.Name

	listOfContainers, err := kc.jobsToContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}
	listOfInitContainers, err := kc.jobsToInitContainers(igName, ig.Jobs, namespace)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}

	_, interpolatedManifestSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeManifestAndVars,
		kc.manifestName,
		VarInterpolationContainerName,
		true,
	)
	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		kc.manifestName,
		ig.Name,
		true,
	)

	volumes := []corev1.Volume{
		{
			Name:         "rendering-data",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name:         "jobs-dir",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name: generateVolumeName(interpolatedManifestSecretName),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: interpolatedManifestSecretName,
				},
			},
		},
		{
			Name: generateVolumeName(resolvedPropertiesSecretName),
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: resolvedPropertiesSecretName,
				},
			},
		},
	}

	eJob := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", kc.manifestName, igName),
			Namespace: namespace,
			Labels: map[string]string{
				LabelInstanceGroupName: igName,
			},
		},
		Spec: ejv1.ExtendedJobSpec{
			UpdateOnConfigChange: true,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: igName,
					Labels: map[string]string{
						"delete": "pod",
					},
				},
				Spec: corev1.PodSpec{
					Containers:     listOfContainers,
					InitContainers: listOfInitContainers,
					Volumes:        volumes,
				},
			},
		},
	}
	return eJob, nil
}

// convertToExtendedJob will convert instance_groups which lifecycle is
// errand to ExtendedJobs
func (kc *KubeConfig) convertToExtendedJob(instanceGroups []*InstanceGroup) ([]ejv1.ExtendedJob, error) {
	eJobs := []ejv1.ExtendedJob{}
	for _, ig := range instanceGroups {
		if ig.LifeCycle == "errand" {
			convertedEJob, err := kc.errandToExtendedJob(ig, kc.namespace)
			if err != nil {
				return []ejv1.ExtendedJob{}, err
			}
			eJobs = append(eJobs, convertedEJob)
		}
	}
	return eJobs, nil
}

func (kc *KubeConfig) convertVariables(variables []Variable) []esv1.ExtendedSecret {
	secrets := []esv1.ExtendedSecret{}

	for _, v := range variables {
		secretName := names.CalculateSecretName(names.DeploymentSecretTypeGeneratedVariable, kc.manifestName, v.Name)
		s := esv1.ExtendedSecret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: kc.namespace,
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
					Name: names.CalculateSecretName(names.DeploymentSecretTypeGeneratedVariable, kc.manifestName, v.Options.CA),
					Key:  "certificate",
				}
				certRequest.CAKeyRef = esv1.SecretReference{
					Name: names.CalculateSecretName(names.DeploymentSecretTypeGeneratedVariable, kc.manifestName, v.Options.CA),
					Key:  "private_key",
				}
			}
			s.Spec.Request.CertificateRequest = certRequest
		}
		secrets = append(secrets, s)
	}

	return secrets
}

// jobSpecCopierContainer will return a corev1.Container{} with the populated field
func jobSpecCopierContainer(releaseName string, releaseImage string, volumeMountName string) corev1.Container {
	inContainerReleasePath := filepath.Join("/var/vcap/all-releases/jobs-src", releaseName)
	initContainers := corev1.Container{
		Name:  fmt.Sprintf("spec-copier-%s", releaseName),
		Image: releaseImage,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeMountName,
				MountPath: "/var/vcap/all-releases",
			},
		},
		Command: []string{
			"bash",
			"-c",
			fmt.Sprintf(`mkdir -p "%s" && cp -ar /var/vcap/jobs-src/* "%s"`, inContainerReleasePath, inContainerReleasePath),
		},
	}

	return initContainers
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
