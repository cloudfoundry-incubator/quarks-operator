package bpmconverter

import (
	"strconv"

	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpm"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/boshdns"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/names"
	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-statefulset/pkg/kube/controllers/statefulset"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

var (
	admGroupID = int64(1000)
)

// BPMConverter converts BPM information to kubernetes resources
type BPMConverter struct {
	volumeFactory           VolumeFactory
	newContainerFactoryFunc NewContainerFactoryFunc
}

// ContainerFactory builds Kubernetes containers from BOSH jobs.
type ContainerFactory interface {
	JobsToInitContainers(jobs []bdm.Job, defaultVolumeMounts []corev1.VolumeMount, bpmDisks bdm.Disks, requiredService *string) ([]corev1.Container, error)
	JobsToContainers(jobs []bdm.Job, defaultVolumeMounts []corev1.VolumeMount, bpmDisks bdm.Disks) ([]corev1.Container, error)
}

// NewContainerFactoryFunc returns ContainerFactory from single BOSH instance group.
type NewContainerFactoryFunc func(instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider bdm.ReleaseImageProvider, bpmConfigs bpm.Configs) ContainerFactory

// NewContainerFactoryImplFunc returns a ContainerFactoryImpl
func NewContainerFactoryImplFunc(instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider bdm.ReleaseImageProvider, bpmConfigs bpm.Configs) ContainerFactory {
	return NewContainerFactory(instanceGroupName, version, disableLogSidecar, releaseImageProvider, bpmConfigs)
}

// VolumeFactory builds Kubernetes containers from BOSH jobs.
type VolumeFactory interface {
	GenerateDefaultDisks(instanceGroupName *bdm.InstanceGroup, igResolvedSecretVersion string, namespace string) bdm.Disks
	GenerateBPMDisks(instanceGroup *bdm.InstanceGroup, bpmConfigs bpm.Configs, namespace string) (bdm.Disks, error)
}

// DNSSettings is a limited interface for the funcs used in the bpm converter
type DNSSettings interface {
	// DNSSetting get the DNS settings for POD.
	DNSSetting(namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error)
}

// NewConverter returns a new converter
func NewConverter(volumeFactory VolumeFactory, newContainerFactoryFunc NewContainerFactoryFunc) *BPMConverter {
	return &BPMConverter{
		volumeFactory:           volumeFactory,
		newContainerFactoryFunc: newContainerFactoryFunc,
	}
}

// Resources contains BPM related k8s resources, which were converted from BOSH objects
type Resources struct {
	InstanceGroups         []qstsv1a1.QuarksStatefulSet
	Errands                []qjv1a1.QuarksJob
	Services               []corev1.Service
	PersistentVolumeClaims []corev1.PersistentVolumeClaim
}

// FilterLabels filters out labels, that are not suitable for StatefulSet updates
func FilterLabels(labels map[string]string) map[string]string {
	statefulSetLabels := make(map[string]string)
	for key, value := range labels {
		if key != bdv1.LabelDeploymentVersion {
			statefulSetLabels[key] = value
		}
	}
	return statefulSetLabels
}

// Resources uses BOSH Process Manager information to create k8s container specs from single BOSH instance group.
// It returns quarks stateful sets, services and quarks jobs.
func (kc *BPMConverter) Resources(manifest bdm.Manifest, namespace string, deploymentName string, serviceIP string, qStsVersion string, instanceGroup *bdm.InstanceGroup, bpmConfigs bpm.Configs, igResolvedSecretVersion string) (*Resources, error) {
	instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Set(deploymentName, instanceGroup.Name, qStsVersion)

	defaultDisks := kc.volumeFactory.GenerateDefaultDisks(instanceGroup, igResolvedSecretVersion, namespace)
	bpmDisks, err := kc.volumeFactory.GenerateBPMDisks(instanceGroup, bpmConfigs, namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Generate of BPM disks failed for manifest name %s, instance group %s.", deploymentName, instanceGroup.Name)
	}

	// Add any special disks to the list of BPM disks
	bpmDisks = append(bpmDisks, instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Disks...)

	allDisks := append(defaultDisks, bpmDisks...)

	res := &Resources{
		PersistentVolumeClaims: allDisks.PVCs(),
	}

	cfac := kc.newContainerFactoryFunc(
		instanceGroup.Name,
		igResolvedSecretVersion,
		instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.DisableLogSidecar,
		&manifest,
		bpmConfigs,
	)

	switch instanceGroup.LifeCycle {
	case bdm.IGTypeService, "":
		convertedExtStatefulSet, err := kc.serviceToQuarksStatefulSet(manifest, namespace, cfac, serviceIP, instanceGroup, defaultDisks, bpmDisks, bpmConfigs.ActivePassiveProbes())
		if err != nil {
			return nil, err
		}

		services := kc.serviceToKubeServices(namespace, deploymentName, instanceGroup, &convertedExtStatefulSet, bpmConfigs)
		if len(services) != 0 {
			res.Services = append(res.Services, services...)
		}

		res.InstanceGroups = append(res.InstanceGroups, convertedExtStatefulSet)
	case bdm.IGTypeErrand, bdm.IGTypeAutoErrand:
		convertedQJob, err := kc.errandToQuarksJob(manifest, namespace, cfac, serviceIP, instanceGroup, defaultDisks, bpmDisks)
		if err != nil {
			return nil, err
		}

		res.Errands = append(res.Errands, convertedQJob)
	}

	return res, nil
}

// serviceToQuarksStatefulSet will generate an QuarksStatefulSet
func (kc *BPMConverter) serviceToQuarksStatefulSet(
	manifest bdm.Manifest,
	namespace string,
	cfac ContainerFactory,
	serviceIP string,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks bdm.Disks,
	bpmDisks bdm.Disks,
	activePassiveProbes map[string]corev1.Probe,
) (qstsv1a1.QuarksStatefulSet, error) {
	defaultVolumeMounts := defaultDisks.VolumeMounts()
	initContainers, err := cfac.JobsToInitContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks, instanceGroup.Properties.Quarks.RequiredService)
	if err != nil {
		return qstsv1a1.QuarksStatefulSet{}, errors.Wrapf(err, "building initContainers failed for instance group %s", instanceGroup.Name)
	}

	containers, err := cfac.JobsToContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return qstsv1a1.QuarksStatefulSet{}, errors.Wrapf(err, "building containers failed for instance group %s", instanceGroup.Name)
	}

	defaultVolumes := defaultDisks.Volumes()
	bpmVolumes := bpmDisks.Volumes()
	volumes := make([]corev1.Volume, 0, len(defaultVolumes)+len(bpmVolumes))
	volumes = append(volumes, defaultVolumes...)
	volumes = append(volumes, bpmVolumes...)

	defaultVolumeClaims := defaultDisks.PVCs()
	bpmVolumeClaims := bpmDisks.PVCs()
	volumeClaims := make([]corev1.PersistentVolumeClaim, 0, len(defaultVolumeClaims)+len(bpmVolumeClaims))
	volumeClaims = append(volumeClaims, defaultVolumeClaims...)
	volumeClaims = append(volumeClaims, bpmVolumeClaims...)

	statefulSetLabels := FilterLabels(instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels)
	statefulSetAnnotations, err := computeAnnotations(instanceGroup)
	if err != nil {
		return qstsv1a1.QuarksStatefulSet{}, errors.Wrapf(err, "computing annotations failed for instance group %s", instanceGroup.Name)
	}
	extSts := qstsv1a1.QuarksStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instanceGroup.NameSanitized(),
			Namespace:   namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: qstsv1a1.QuarksStatefulSetSpec{
			Zones:                instanceGroup.AZs,
			UpdateOnConfigChange: true,
			ActivePassiveProbes:  activePassiveProbes,
			InjectReplicasEnv:    instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.InjectReplicasEnv,
			Template: appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        instanceGroup.NameSanitized(),
					Labels:      statefulSetLabels,
					Annotations: statefulSetAnnotations,
				},
				Spec: appsv1.StatefulSetSpec{
					Replicas: pointers.Int32(int32(instanceGroup.Instances)),
					Selector: &metav1.LabelSelector{
						MatchLabels: statefulSetLabels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels:      statefulSetLabels,
							Name:        instanceGroup.NameSanitized(),
							Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
						},
						Spec: corev1.PodSpec{
							TerminationGracePeriodSeconds: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.TerminationGracePeriodSeconds,
							Affinity:                      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity,
							Volumes:                       volumes,
							InitContainers:                initContainers,
							Containers:                    containers,
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup: &admGroupID,
							},
							Subdomain:        names.ServiceName(instanceGroup.Name),
							ImagePullSecrets: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ImagePullSecrets,
						},
					},
					VolumeClaimTemplates: volumeClaims,
				},
			},
		},
	}

	spec := &extSts.Spec.Template.Spec.Template.Spec

	spec.DNSPolicy, spec.DNSConfig, err = boshdns.DNSSetting(manifest, serviceIP, namespace)
	if err != nil {
		return qstsv1a1.QuarksStatefulSet{}, err
	}

	if len(instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Tolerations) > 0 {
		extSts.Spec.Template.Spec.Template.Spec.Tolerations = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Tolerations
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName != "" {
		extSts.Spec.Template.Spec.Template.Spec.ServiceAccountName = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken != nil {
		extSts.Spec.Template.Spec.Template.Spec.AutomountServiceAccountToken = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken
	}

	return extSts, nil
}

// serviceToKubeServices will generate Services which expose ports for InstanceGroup's jobs
func (kc *BPMConverter) serviceToKubeServices(namespace string, deploymentName string, instanceGroup *bdm.InstanceGroup, qSts *qstsv1a1.QuarksStatefulSet, bpmConfigs bpm.Configs) []corev1.Service {
	var services []corev1.Service
	// Collect ports from bpm configs
	ports := bpmConfigs.ServicePorts()
	if len(ports) == 0 {
		return services
	}

	isActivePassiveModel := bpmConfigs.IsActivePassiveModel()

	if len(instanceGroup.AZs) > 0 {
		for azIndex := range instanceGroup.AZs {
			services = kc.generateServices(
				services,
				namespace,
				deploymentName,
				*instanceGroup,
				azIndex,
				isActivePassiveModel,
				ports)
		}
	} else {
		services = kc.generateServices(
			services,
			namespace,
			deploymentName,
			*instanceGroup,
			-1,
			isActivePassiveModel,
			ports)
	}

	headlessServiceName := names.ServiceName(instanceGroup.Name)
	headlessServiceSelector := map[string]string{
		bdv1.LabelDeploymentName:    deploymentName,
		bdv1.LabelInstanceGroupName: instanceGroup.Name,
	}
	if isActivePassiveModel {
		headlessServiceSelector[qstsv1a1.LabelActivePod] = "active"
	}

	labels := labels.Merge(
		instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
		map[string]string{bdv1.LabelInstanceGroupName: instanceGroup.Name},
	)
	headlessService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        headlessServiceName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: corev1.ServiceSpec{
			Ports:     ports,
			Selector:  headlessServiceSelector,
			ClusterIP: "None",
		},
	}

	// Set headlessService to govern StatefulSet.
	qSts.Spec.Template.Spec.ServiceName = headlessServiceName

	services = append(services, headlessService)

	return services
}

// errandToQuarksJob will generate an QuarksJob
func (kc *BPMConverter) errandToQuarksJob(
	manifest bdm.Manifest,
	namespace string,
	cfac ContainerFactory,
	serviceIP string,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks bdm.Disks,
	bpmDisks bdm.Disks,
) (qjv1a1.QuarksJob, error) {
	defaultVolumeMounts := defaultDisks.VolumeMounts()
	initContainers, err := cfac.JobsToInitContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks, instanceGroup.Properties.Quarks.RequiredService)
	if err != nil {
		return qjv1a1.QuarksJob{}, errors.Wrapf(err, "building initContainers failed for instance group %s", instanceGroup.Name)
	}

	containers, err := cfac.JobsToContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return qjv1a1.QuarksJob{}, errors.Wrapf(err, "building containers failed for instance group %s", instanceGroup.Name)
	}

	podLabels := instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels

	defaultVolumes := defaultDisks.Volumes()
	bpmVolumes := bpmDisks.Volumes()
	volumes := make([]corev1.Volume, 0, len(defaultVolumes)+len(bpmVolumes))
	volumes = append(volumes, defaultVolumes...)
	volumes = append(volumes, bpmVolumes...)

	strategy := qjv1a1.TriggerManual
	if instanceGroup.LifeCycle == bdm.IGTypeAutoErrand {
		strategy = qjv1a1.TriggerOnce
	}

	restartPolicy := corev1.RestartPolicyOnFailure
	if instanceGroup.LifeCycle == bdm.IGTypeErrand {
		// Manually triggered errands are not auto-restarted
		restartPolicy = corev1.RestartPolicyNever
	}

	qJob := qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instanceGroup.Name,
			Namespace:   namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: qjv1a1.QuarksJobSpec{
			Trigger: qjv1a1.Trigger{
				Strategy: strategy,
			},
			Template: batchv1b1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					BackoffLimit: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.JobBackoffLimit,
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:        instanceGroup.Name,
							Labels:      podLabels,
							Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
						},
						Spec: corev1.PodSpec{
							RestartPolicy:  restartPolicy,
							Containers:     containers,
							InitContainers: initContainers,
							Volumes:        volumes,
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup: &admGroupID,
							},
							ImagePullSecrets: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ImagePullSecrets,
						},
					},
				},
			},
		},
	}

	qJob.Spec.Template.Spec.Template.Spec.DNSPolicy, qJob.Spec.Template.Spec.Template.Spec.DNSConfig, err = boshdns.DNSSetting(manifest, serviceIP, namespace)
	if err != nil {
		return qjv1a1.QuarksJob{}, err
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity != nil {
		qJob.Spec.Template.Spec.Template.Spec.Affinity = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity
	}

	if len(instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Tolerations) > 0 {
		qJob.Spec.Template.Spec.Template.Spec.Tolerations = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Tolerations
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName != "" {
		qJob.Spec.Template.Spec.Template.Spec.ServiceAccountName = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken != nil {
		qJob.Spec.Template.Spec.Template.Spec.AutomountServiceAccountToken = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken
	}

	return qJob, nil
}

func (kc *BPMConverter) generateServices(
	services []corev1.Service,
	namespace string,
	deploymentName string,
	instanceGroup bdm.InstanceGroup,
	azIndex int,
	activePassiveModel bool,
	ports []corev1.ServicePort) []corev1.Service {
	serviceLabels := func(azIndex, ordinal int, includeActiveSelector bool) map[string]string {
		if azIndex == -1 {
			azIndex = 0
		}
		labels := map[string]string{
			bdv1.LabelDeploymentName:    deploymentName,
			bdv1.LabelInstanceGroupName: instanceGroup.Name,
			qstsv1a1.LabelAZIndex:       strconv.Itoa(azIndex),
			qstsv1a1.LabelPodOrdinal:    strconv.Itoa(ordinal),
		}
		if includeActiveSelector {
			labels[qstsv1a1.LabelActivePod] = "active"
		}
		return labels
	}

	for i := 0; i < instanceGroup.Instances; i++ {
		services = append(services, corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceGroup.IndexedServiceName(i, azIndex),
				Namespace: namespace,
				Labels:    serviceLabels(azIndex, i, false),
			},
			Spec: corev1.ServiceSpec{
				Ports:    ports,
				Selector: serviceLabels(azIndex, i, activePassiveModel),
			},
		})
	}

	return services
}

// computeAnnotations computes annotations for the statefulset from the instance group
func computeAnnotations(ig *bdm.InstanceGroup) (map[string]string, error) {
	statefulSetAnnotations := ig.Env.AgentEnvBoshConfig.Agent.Settings.Annotations
	if statefulSetAnnotations == nil {
		statefulSetAnnotations = make(map[string]string)
	}
	if ig.Update == nil {
		return statefulSetAnnotations, nil
	}

	canaryWatchTime, err := bdm.ExtractWatchTime(ig.Update.CanaryWatchTime)
	if err != nil {
		return nil, errors.Wrap(err, "update block has invalid canary_watch_time")
	}
	if canaryWatchTime != "" {
		statefulSetAnnotations[statefulset.AnnotationCanaryWatchTime] = canaryWatchTime
	}

	updateWatchTime, err := bdm.ExtractWatchTime(ig.Update.UpdateWatchTime)
	if err != nil {
		return nil, errors.Wrap(err, "update block has invalid update_watch_time")
	}

	if updateWatchTime != "" {
		statefulSetAnnotations[statefulset.AnnotationUpdateWatchTime] = updateWatchTime
	}

	return statefulSetAnnotations, nil
}
