package converter

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
)

var (
	vcapUserID int64 = 1000
	admGroupID int64 = 1000
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
	Disks          BPMResourceDisks
}

// BPMResourceDisk represents a converted BPM disk to k8s resources.
type BPMResourceDisk struct {
	PersistentVolumeClaim *corev1.PersistentVolumeClaim
	Volume                *corev1.Volume
	VolumeMount           *corev1.VolumeMount

	Labels map[string]string
}

// MatchesFilter returns true if the disk matches the filter with one of its labels.
func (disk *BPMResourceDisk) MatchesFilter(filterKey, filterValue string) bool {
	labelValue, exists := disk.Labels[filterKey]
	if !exists {
		return false
	}
	return labelValue == filterValue
}

// BPMResourceDisks represents a slice of BPMResourceDisk.
type BPMResourceDisks []BPMResourceDisk

// Filter filters BPMResourceDisks on its labels.
func (disks BPMResourceDisks) Filter(filterKey, filterValue string) BPMResourceDisks {
	filtered := make(BPMResourceDisks, 0)
	for _, disk := range disks {
		if disk.MatchesFilter(filterKey, filterValue) {
			filtered = append(filtered, disk)
		}
	}
	return filtered
}

// VolumeMounts returns a slice of VolumeMount of each BPMResourceDisk contained in BPMResourceDisks.
func (disks BPMResourceDisks) VolumeMounts() []corev1.VolumeMount {
	volumeMounts := make([]corev1.VolumeMount, 0)
	for _, disk := range disks {
		if disk.VolumeMount != nil {
			volumeMounts = append(volumeMounts, *disk.VolumeMount)
		}
	}
	return volumeMounts
}

// Volumes returns a slice of Volume of each BPMResourceDisk contained in BPMResourceDisks.
func (disks BPMResourceDisks) Volumes() []corev1.Volume {
	volumes := make([]corev1.Volume, 0)
	for _, disk := range disks {
		if disk.Volume != nil {
			volumes = append(volumes, *disk.Volume)
		}
	}
	return volumes
}

// BPMResources uses BOSH Process Manager information to create k8s container specs from single BOSH instance group.
// It returns extended stateful sets, services and extended jobs.
func (kc *KubeConverter) BPMResources(manifestName string, version string, instanceGroup *bdm.InstanceGroup, releaseImageProvider ReleaseImageProvider, bpmConfigs bpm.Configs) (*BPMResources, error) {
	instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Set(manifestName, instanceGroup.Name, version)

	defaultDisks := generateDefaultDisks(manifestName, instanceGroup, version, kc.namespace)

	bpmDisks, err := generateBPMDisks(manifestName, instanceGroup, bpmConfigs, kc.namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Generate of BPM disks failed for manifest name %s, instance group %s.", manifestName, instanceGroup.Name)
	}

	allDisks := append(defaultDisks, bpmDisks...)

	res := &BPMResources{
		Disks: allDisks,
	}

	cfac := NewContainerFactory(
		manifestName,
		instanceGroup.Name,
		version,
		instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.DisableLogSidecar,
		releaseImageProvider,
		bpmConfigs,
	)

	switch instanceGroup.LifeCycle {
	case "service", "":
		convertedExtStatefulSet, err := kc.serviceToExtendedSts(cfac, manifestName, instanceGroup, defaultDisks, bpmDisks)
		if err != nil {
			return nil, err
		}

		services := kc.serviceToKubeServices(manifestName, instanceGroup, &convertedExtStatefulSet)
		if len(services) != 0 {
			res.Services = append(res.Services, services...)
		}

		res.InstanceGroups = append(res.InstanceGroups, convertedExtStatefulSet)
	case "errand":
		convertedEJob, err := kc.errandToExtendedJob(cfac, manifestName, instanceGroup, defaultDisks, bpmDisks)
		if err != nil {
			return nil, err
		}

		res.Errands = append(res.Errands, convertedEJob)
	}

	return res, nil
}

// serviceToExtendedSts will generate an ExtendedStatefulSet
func (kc *KubeConverter) serviceToExtendedSts(
	cfac *ContainerFactory,
	manifestName string,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks BPMResourceDisks,
	bpmDisks BPMResourceDisks,
) (essv1.ExtendedStatefulSet, error) {
	defaultVolumeMounts := defaultDisks.VolumeMounts()
	initContainers, err := cfac.JobsToInitContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, errors.Wrapf(err, "Generating extendstatefulset failed for manifest %s", manifestName)
	}

	containers, err := cfac.JobsToContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, errors.Wrapf(err, "Generating extendstatefulset failed for manifest %s", manifestName)
	}

	defaultVolumes := defaultDisks.Volumes()
	bpmVolumes := bpmDisks.Volumes()
	volumes := make([]corev1.Volume, 0, len(defaultVolumes)+len(bpmVolumes))
	volumes = append(volumes, defaultVolumes...)
	volumes = append(volumes, bpmVolumes...)

	extSts := essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instanceGroup.ExtendedStatefulsetName(manifestName),
			Namespace:   kc.namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			UpdateOnConfigChange: true,
			Template: v1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        instanceGroup.NameSanitized(),
					Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
					Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
				},
				Spec: v1beta2.StatefulSetSpec{
					Replicas: util.Int32(int32(instanceGroup.Instances)),
					Selector: &metav1.LabelSelector{
						MatchLabels: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
							Name:        instanceGroup.NameSanitized(),
							Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
						},
						Spec: corev1.PodSpec{
							Affinity:       instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity,
							Volumes:        volumes,
							InitContainers: initContainers,
							Containers:     containers,
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup: &admGroupID,
							},
							Subdomain: instanceGroup.HeadlessServiceName(manifestName),
						},
					},
				},
			},
		},
	}

	return extSts, nil
}

// serviceToKubeServices will generate Services which expose ports for InstanceGroup's jobs
func (kc *KubeConverter) serviceToKubeServices(manifestName string, instanceGroup *bdm.InstanceGroup, eSts *essv1.ExtendedStatefulSet) []corev1.Service {
	var services []corev1.Service
	// Collect ports to be exposed for each job
	ports := []corev1.ServicePort{}
	for _, job := range instanceGroup.Jobs {
		for _, port := range job.Properties.Quarks.Ports {
			ports = append(ports, corev1.ServicePort{
				Name:     port.Name,
				Protocol: corev1.Protocol(port.Protocol),
				Port:     int32(port.Internal),
			})
		}
	}

	if len(ports) == 0 {
		return services
	}

	for i := 0; i < instanceGroup.Instances; i++ {
		if len(instanceGroup.AZs) == 0 {
			services = append(services, corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceGroup.IndexedServiceName(manifestName, len(services)),
					Namespace: kc.namespace,
					Labels: map[string]string{
						bdm.LabelDeploymentName:    manifestName,
						bdm.LabelInstanceGroupName: instanceGroup.Name,
						essv1.LabelAZIndex:         strconv.Itoa(0),
						essv1.LabelPodOrdinal:      strconv.Itoa(i),
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: ports,
					Selector: map[string]string{
						bdm.LabelInstanceGroupName: instanceGroup.Name,
						essv1.LabelAZIndex:         strconv.Itoa(0),
						essv1.LabelPodOrdinal:      strconv.Itoa(i),
					},
				},
			})
		}
		for azIndex := range instanceGroup.AZs {
			services = append(services, corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      instanceGroup.IndexedServiceName(manifestName, len(services)),
					Namespace: kc.namespace,
					Labels: map[string]string{
						bdm.LabelInstanceGroupName: instanceGroup.Name,
						essv1.LabelAZIndex:         strconv.Itoa(azIndex),
						essv1.LabelPodOrdinal:      strconv.Itoa(i),
					},
				},
				Spec: corev1.ServiceSpec{
					Ports: ports,
					Selector: map[string]string{
						bdm.LabelInstanceGroupName: instanceGroup.Name,
						essv1.LabelAZIndex:         strconv.Itoa(azIndex),
						essv1.LabelPodOrdinal:      strconv.Itoa(i),
					},
				},
			})
		}
	}

	headlessService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instanceGroup.HeadlessServiceName(manifestName),
			Namespace:   kc.namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports: ports,
			Selector: map[string]string{
				bdm.LabelInstanceGroupName: instanceGroup.Name,
			},
			ClusterIP: "None",
		},
	}

	services = append(services, headlessService)

	// Set headlessService to govern StatefulSet.
	eSts.Spec.Template.Spec.ServiceName = headlessService.ObjectMeta.Name

	return services
}

// errandToExtendedJob will generate an ExtendedJob
func (kc *KubeConverter) errandToExtendedJob(
	cfac *ContainerFactory,
	manifestName string,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks BPMResourceDisks,
	bpmDisks BPMResourceDisks,
) (ejv1.ExtendedJob, error) {
	defaultVolumeMounts := defaultDisks.VolumeMounts()
	initContainers, err := cfac.JobsToInitContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return ejv1.ExtendedJob{}, errors.Wrapf(err, "Generating extended job failed for manifest %s", manifestName)
	}

	containers, err := cfac.JobsToContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return ejv1.ExtendedJob{}, errors.Wrapf(err, "Generating extended job failed for manifest %s", manifestName)
	}

	podLabels := instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels
	// Controller will delete successful job
	podLabels["delete"] = "pod"

	defaultVolumes := defaultDisks.Volumes()
	bpmVolumes := bpmDisks.Volumes()
	volumes := make([]corev1.Volume, 0, len(defaultVolumes)+len(bpmVolumes))
	volumes = append(volumes, defaultVolumes...)
	volumes = append(volumes, bpmVolumes...)

	// Errand EJob
	eJob := ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", manifestName, instanceGroup.Name),
			Namespace:   kc.namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerManual,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        instanceGroup.Name,
					Labels:      podLabels,
					Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
				},
				Spec: corev1.PodSpec{
					Containers:     containers,
					InitContainers: initContainers,
					Volumes:        volumes,
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup: &admGroupID,
					},
				},
			},
		},
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity != nil {
		eJob.Spec.Template.Spec.Affinity = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity
	}
	return eJob, nil
}

func deduplicateVolumeMounts(volumeMounts []corev1.VolumeMount) []corev1.VolumeMount {
	result := []corev1.VolumeMount{}
	uniqueMounts := map[string]struct{}{}

	for _, volumeMount := range volumeMounts {
		if _, ok := uniqueMounts[volumeMount.MountPath]; ok {
			continue
		}

		uniqueMounts[volumeMount.MountPath] = struct{}{}
		result = append(result, volumeMount)
	}

	return result
}
