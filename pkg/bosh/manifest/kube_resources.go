package manifest

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

const (
	// VarInterpolationContainerName is the name of the container that performs
	// variable interpolation for a manifest
	VarInterpolationContainerName = "interpolation"
)

var (
	// LabelDeploymentName is the name of a label for the deployment name
	LabelDeploymentName = fmt.Sprintf("%s/deployment-name", apis.GroupName)
	// LabelInstanceGroupName is the name of a label for an instance group name
	LabelInstanceGroupName = fmt.Sprintf("%s/instance-group-name", apis.GroupName)
	// AnnotationDeploymentVersion is the annotation key for deployment version
	AnnotationDeploymentVersion = fmt.Sprintf("%s/deployment-version", apis.GroupName)
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

// BPMResources uses BOSH Process Manager information to create k8s container specs from BOSH instance groups.
// It returns extended stateful sets, services and extended jobs.
func (kc *KubeConverter) BPMResources(manifestName string, version string, instanceGroups []*InstanceGroup, releaseImageProvider ReleaseImageProvider, allBPMConfigs map[string]bpm.Configs) (*BPMResources, error) {
	res := &BPMResources{}

	for _, ig := range instanceGroups {

		bpmConfigs, ok := allBPMConfigs[ig.Name]
		if !ok {
			return nil, errors.Errorf("couldn't find instance group '%s' in bpm configs set", ig.Name)
		}
		cfac := NewContainerFactory(manifestName, ig.Name, releaseImageProvider, bpmConfigs)

		switch ig.LifeCycle {
		case "service", "":
			convertedExtStatefulSet, err := kc.serviceToExtendedSts(manifestName, version, ig, cfac)
			if err != nil {
				return nil, err
			}

			// Create a persistent volume claim if specified in spec
			if ig.PersistentDisk != nil {
				if *ig.PersistentDisk > 0 {
					persistentVolumeClaim := kc.diskToPersistentVolumeClaims(cfac, &convertedExtStatefulSet, manifestName, ig)
					res.Disks = append(res.Disks, *persistentVolumeClaim)

					// Add volumes spec to pod spec and container spec of pvc in extendedstatefulset
					convertedExtStatefulSet = *kc.addPVCVolumeSpecs(cfac, &convertedExtStatefulSet, manifestName, ig)
				}
			}

			services, err := kc.serviceToKubeServices(manifestName, version, ig, &convertedExtStatefulSet)
			if len(services) != 0 {
				res.Services = append(res.Services, services...)
			}

			res.InstanceGroups = append(res.InstanceGroups, convertedExtStatefulSet)
		case "errand":
			convertedEJob, err := kc.errandToExtendedJob(manifestName, version, ig, cfac)
			if err != nil {
				return nil, err
			}
			res.Errands = append(res.Errands, convertedEJob)
		}
	}

	return res, nil
}

// serviceToExtendedSts will generate an ExtendedStatefulSet
func (kc *KubeConverter) serviceToExtendedSts(manifestName string, version string, ig *InstanceGroup, cfac *ContainerFactory) (essv1.ExtendedStatefulSet, error) {
	igName := ig.Name

	listOfInitContainers, err := cfac.JobsToInitContainers(ig.Jobs)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	_, interpolatedManifestSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeManifestAndVars,
		manifestName,
		VarInterpolationContainerName,
		true,
	)
	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		manifestName,
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
		// for ephemeral job data
		// https://bosh.io/docs/vm-config/#jobs-and-packages
		{
			Name:         "data-dir",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
		},
		{
			Name:         "sys-dir",
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

	containers, err := cfac.JobsToContainers(ig.Jobs)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
	}

	extSts := essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", manifestName, igName),
			Namespace: kc.namespace,
			Labels: map[string]string{
				LabelDeploymentName:    manifestName,
				LabelInstanceGroupName: igName,
			},
			Annotations: map[string]string{
				AnnotationDeploymentVersion: version,
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
							bdv1.LabelDeploymentName: manifestName,
							LabelInstanceGroupName:   igName,
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: igName,
							Labels: map[string]string{
								bdv1.LabelDeploymentName: manifestName,
								LabelInstanceGroupName:   igName,
							},
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
			Name:      names.ServiceName(manifestName, igName, -1),
			Namespace: kc.namespace,
			Labels: map[string]string{
				LabelInstanceGroupName: igName,
			},
			Annotations: map[string]string{
				AnnotationDeploymentVersion: version,
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
	eSts.Spec.Template.Spec.ServiceName = names.ServiceName(manifestName, igName, -1)

	return services, nil
}

// errandToExtendedJob will generate an ExtendedJob
func (kc *KubeConverter) errandToExtendedJob(manifestName string, version string, ig *InstanceGroup, cfac *ContainerFactory) (ejv1.ExtendedJob, error) {
	igName := ig.Name

	listOfInitContainers, err := cfac.JobsToInitContainers(ig.Jobs)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}

	_, interpolatedManifestSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeManifestAndVars,
		manifestName,
		VarInterpolationContainerName,
		true,
	)
	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		manifestName,
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

	containers, err := cfac.JobsToContainers(ig.Jobs)
	if err != nil {
		return ejv1.ExtendedJob{}, err
	}

	eJob := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", manifestName, igName),
			Namespace: kc.namespace,
			Labels: map[string]string{
				LabelDeploymentName:    manifestName,
				LabelInstanceGroupName: igName,
			},
			Annotations: map[string]string{
				AnnotationDeploymentVersion: version,
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
					Containers:     containers,
					InitContainers: listOfInitContainers,
					Volumes:        volumes,
				},
			},
		},
	}
	return eJob, nil
}

func (kc *KubeConverter) diskToPersistentVolumeClaims(cfac *ContainerFactory, extendedStatefulset *essv1.ExtendedStatefulSet, manifestName string, ig *InstanceGroup) *corev1.PersistentVolumeClaim {

	// spec of a persistent volumeclaim
	persistentVolumeClaim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-%s", manifestName, ig.Name, "pvc"),
			Namespace: kc.namespace,
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

func (kc *KubeConverter) addPVCVolumeSpecs(cfac *ContainerFactory, extendedStatefulset *essv1.ExtendedStatefulSet, manifestName string, ig *InstanceGroup) *essv1.ExtendedStatefulSet {
	// add volumeMount specs to container of Extendedstatefulset
	storeVolume := corev1.VolumeMount{
		Name:      "store-dir",
		MountPath: "/var/vcap/store",
	}
	containers := extendedStatefulset.Spec.Template.Spec.Template.Spec.Containers
	for containerIndex, container := range containers {
		containers[containerIndex].VolumeMounts = append(container.VolumeMounts, storeVolume)
	}
	extendedStatefulset.Spec.Template.Spec.Template.Spec.Containers = containers

	// add volume spec to pod volumes of Extendedstatefulset
	pvcVolume := corev1.Volume{
		Name: "store-dir",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: fmt.Sprintf("%s-%s-%s", manifestName, ig.Name, "pvc"),
			},
		},
	}
	extendedStatefulset.Spec.Template.Spec.Template.Spec.Volumes = append(extendedStatefulset.Spec.Template.Spec.Template.Spec.Volumes, pvcVolume)
	return extendedStatefulset
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
