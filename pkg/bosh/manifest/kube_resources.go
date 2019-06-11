package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

const (
	// VarInterpolationContainerName is the name of the container that performs
	// variable interpolation for a manifest
	VarInterpolationContainerName = "interpolation"
)

// ReleaseImageProvider interface to provide the docker release image for a BOSH job
// This lookup is currently implemented by the manifest model.
type ReleaseImageProvider interface {
	// GetReleaseImage returns the release image for an job in an instance group
	GetReleaseImage(instanceGroupName, jobName string) (string, error)
}

// BPMResources contains BPM related k8s resources, which were converted from BOSH objects
type BPMResources struct {
	InstanceGroups []essv1.ExtendedStatefulSet
	Errands        []ejv1.ExtendedJob
	Services       []corev1.Service
	Disks          []corev1.PersistentVolumeClaim
}

// BPMResources uses BOSH Process Manager information to create k8s container specs from single BOSH instance group.
// It returns extended stateful sets, services and extended jobs.
func (kc *KubeConverter) BPMResources(manifestName string, version string, instanceGroup *InstanceGroup, releaseImageProvider ReleaseImageProvider, bpmConfigs bpm.Configs) (*BPMResources, error) {
	res := &BPMResources{}

	instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Set(manifestName, instanceGroup.Name, version)

	cfac := NewContainerFactory(manifestName, instanceGroup.Name, releaseImageProvider, bpmConfigs)

	// Create a persistent volume claim if specified in spec.
	if instanceGroup.PersistentDisk != nil && *instanceGroup.PersistentDisk > 0 {
		// Annotations are added to specify the volume name and mount path.
		annotations := map[string]string{
			"volume-name":       VolumeStoreDirName,
			"volume-mount-path": VolumeStoreDirMountPath,
		}
		persistentVolumeClaim := kc.diskToPersistentVolumeClaims(cfac, manifestName, instanceGroup, annotations)
		res.Disks = append(res.Disks, *persistentVolumeClaim)
	}

	switch instanceGroup.LifeCycle {
	case "service", "":
		convertedExtStatefulSet, err := kc.serviceToExtendedSts(manifestName, version, instanceGroup, cfac)
		if err != nil {
			return nil, err
		}

		// Add volumes spec to pod spec and container spec of pvc's in extendedstatefulset
		kc.addPVCSpecs(cfac, &convertedExtStatefulSet, manifestName, instanceGroup, res.Disks)

		services, err := kc.serviceToKubeServices(manifestName, version, instanceGroup, &convertedExtStatefulSet)
		if err != nil {
			return nil, err
		}
		if len(services) != 0 {
			res.Services = append(res.Services, services...)
		}

		res.InstanceGroups = append(res.InstanceGroups, convertedExtStatefulSet)
	case "errand":
		convertedEJob, err := kc.errandToExtendedJob(manifestName, version, instanceGroup, cfac)
		if err != nil {
			return nil, err
		}

		// Add volumes spec to pod spec and container spec of pvc's in extendedJob
		kc.addPVCSpecs(cfac, &convertedEJob, manifestName, instanceGroup, res.Disks)

		res.Errands = append(res.Errands, convertedEJob)
	}

	return res, nil
}

// serviceToExtendedSts will generate an ExtendedStatefulSet
func (kc *KubeConverter) serviceToExtendedSts(manifestName string, version string, ig *InstanceGroup, cfac *ContainerFactory) (essv1.ExtendedStatefulSet, error) {
	igName := ig.Name

	hasPersistentDisk := (ig.PersistentDisk != nil && *ig.PersistentDisk > 0)
	listOfInitContainers, err := cfac.JobsToInitContainers(ig.Jobs, hasPersistentDisk)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	volumes := igVolumes(manifestName, ig.Name)

	containers, err := cfac.JobsToContainers(ig.Jobs)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	bpmVolumes := bpmVolumes(cfac, ig, manifestName)
	volumes = append(volumes, bpmVolumes...)

	extSts := essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", manifestName, igName),
			Namespace:   kc.namespace,
			Labels:      ig.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			UpdateOnConfigChange: true,
			Template: v1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        igName,
					Labels:      ig.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
					Annotations: ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
				},
				Spec: v1beta2.StatefulSetSpec{
					Replicas: func() *int32 { i := int32(ig.Instances); return &i }(),
					Selector: &metav1.LabelSelector{
						MatchLabels: ig.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:        igName,
							Labels:      ig.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
							Annotations: ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
						},
						Spec: corev1.PodSpec{
							Volumes:        volumes,
							InitContainers: listOfInitContainers,
							Containers:     containers,
						},
					},
				},
			},
		},
	}
	return extSts, nil
}

// serviceToKubeServices will generate Services which expose ports for InstanceGroup's jobs
func (kc *KubeConverter) serviceToKubeServices(manifestName string, version string, ig *InstanceGroup, eSts *essv1.ExtendedStatefulSet) ([]corev1.Service, error) {
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
					Name:      names.ServiceName(manifestName, igName, len(services)),
					Namespace: kc.namespace,
					Labels: map[string]string{
						LabelDeploymentName:    manifestName,
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
					Name:      names.ServiceName(manifestName, igName, len(services)),
					Namespace: kc.namespace,
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
			Name:        names.ServiceName(manifestName, igName, -1),
			Namespace:   kc.namespace,
			Labels:      ig.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
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
	eSts.Spec.Template.Spec.ServiceName = names.ServiceName(manifestName, igName, -1)

	return services, nil
}

// errandToExtendedJob will generate an ExtendedJob
func (kc *KubeConverter) errandToExtendedJob(manifestName string, version string, ig *InstanceGroup, cfac *ContainerFactory) (ejv1.ExtendedJob, error) {
	igName := ig.Name

	hasPersistentDisk := (ig.PersistentDisk != nil && *ig.PersistentDisk > 0)
	listOfInitContainers, err := cfac.JobsToInitContainers(ig.Jobs, hasPersistentDisk)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}

	volumes := igVolumes(manifestName, ig.Name)

	containers, err := cfac.JobsToContainers(ig.Jobs)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}

	bpmVolumes := bpmVolumes(cfac, ig, manifestName)
	volumes = append(volumes, bpmVolumes...)

	podLabels := ig.Env.AgentEnvBoshConfig.Agent.Settings.Labels
	// Controller will delete successful job
	podLabels["delete"] = "pod"

	eJob := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", manifestName, igName),
			Namespace:   kc.namespace,
			Labels:      ig.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: ejv1.ExtendedJobSpec{
			UpdateOnConfigChange: true,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        igName,
					Labels:      podLabels,
					Annotations: ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
				},
				Spec: corev1.PodSpec{
					Containers:     containers,
					InitContainers: listOfInitContainers,
					Volumes:        volumes,
				},
			},
		},
	}
	return eJob, nil
}

func (kc *KubeConverter) diskToPersistentVolumeClaims(cfac *ContainerFactory, manifestName string, ig *InstanceGroup, annotations map[string]string) *corev1.PersistentVolumeClaim {
	// spec of a persistent volumeclaim
	persistentVolumeClaim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s-%s", manifestName, ig.Name, "pvc"),
			Namespace:   kc.namespace,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceName(corev1.ResourceStorage): resource.MustParse(fmt.Sprintf("%d%s", *ig.PersistentDisk, "Mi")),
				},
			},
		},
	}

	// add storage class if specified
	if ig.PersistentDiskType != "" {
		persistentVolumeClaim.Spec.StorageClassName = &ig.PersistentDiskType
	}

	return persistentVolumeClaim
}

func (kc *KubeConverter) addPVCSpecs(cfac *ContainerFactory, kubeObject interface{}, manifestName string, ig *InstanceGroup, disks []corev1.PersistentVolumeClaim) {

	claimName := fmt.Sprintf("%s-%s-%s", manifestName, ig.Name, "pvc")

	switch kubeObject := kubeObject.(type) {
	case *essv1.ExtendedStatefulSet:
		kubeObject.Spec.Template.Spec.Template.Spec = kc.addVolumeSpecs(&kubeObject.Spec.Template.Spec.Template.Spec, disks, claimName)
	case *ejv1.ExtendedJob:
		kubeObject.Spec.Template.Spec = kc.addVolumeSpecs(&kubeObject.Spec.Template.Spec, disks, claimName)
	}
}

func (kc *KubeConverter) addVolumeSpecs(podSpec *corev1.PodSpec, disks []corev1.PersistentVolumeClaim, claimName string) corev1.PodSpec {
	for _, disk := range disks {

		// add volumeMount specs to container of Extendedstatefulset
		volumeMountSpec := corev1.VolumeMount{
			Name:      disk.GetAnnotations()["volume-name"],
			MountPath: disk.GetAnnotations()["volume-mount-path"],
		}

		for containerIndex, container := range podSpec.Containers {
			podSpec.Containers[containerIndex].VolumeMounts = append(container.VolumeMounts, volumeMountSpec)
		}

		// add volume spec to pod volumes of Extendedstatefulset
		pvcVolume := corev1.Volume{
			Name: disk.GetAnnotations()["volume-name"],
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: claimName,
				},
			},
		}
		podSpec.Volumes = append(podSpec.Volumes, pvcVolume)
	}
	return *podSpec
}

func bpmVolumes(cfac *ContainerFactory, ig *InstanceGroup, manifestName string) []corev1.Volume {
	var bpmVolumes []corev1.Volume
	for _, job := range ig.Jobs {
		bpmConfig := cfac.bpmConfigs[job.Name]
		for _, process := range bpmConfig.Processes {
			if process.EphemeralDisk {
				eD := corev1.Volume{
					Name:         fmt.Sprintf("%s-%s", VolumeEphemeralDirName, job.Name),
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				}
				bpmVolumes = append(bpmVolumes, eD)
			}

			// TODO: skip this, while we need to figure it out a better way
			// to define persistenVolumeClaims for jobs
			// if process.PersistentDisk {
			// }

			for i := range process.AdditionalVolumes {
				aV := corev1.Volume{
					Name: fmt.Sprintf("%s-%s-%b", AdditionalVolume, job.Name, i),
					VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: fmt.Sprintf("%s-%s-%s", manifestName, ig.Name, "pvc"),
					}},
				}
				bpmVolumes = append(bpmVolumes, aV)
			}

		}
	}
	return bpmVolumes
}

func igVolumes(manifestName, igName string) []corev1.Volume {
	_, interpolatedManifestSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeManifestAndVars,
		manifestName,
		VarInterpolationContainerName,
		true,
	)
	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		manifestName,
		igName,
		true,
	)

	return []corev1.Volume{
		{
			Name:         VolumeRenderingDataName,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name:         VolumeJobsDirName,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		// for ephemeral job data
		// https://bosh.io/docs/vm-config/#jobs-and-packages
		{
			Name:         VolumeDataDirName,
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name:         VolumeSysDirName,
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
