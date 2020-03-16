package bpmconverter

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/disk"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers/statefulset"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

var (
	admGroupID = int64(1000)
)

// BPMConverter converts BPM information to kubernetes resources
type BPMConverter struct {
	namespace               string
	volumeFactory           VolumeFactory
	newContainerFactoryFunc NewContainerFactoryFunc
}

// ContainerFactory builds Kubernetes containers from BOSH jobs.
type ContainerFactory interface {
	JobsToInitContainers(jobs []bdm.Job, defaultVolumeMounts []corev1.VolumeMount, bpmDisks disk.BPMResourceDisks, requiredService *string) ([]corev1.Container, error)
	JobsToContainers(jobs []bdm.Job, defaultVolumeMounts []corev1.VolumeMount, bpmDisks disk.BPMResourceDisks) ([]corev1.Container, error)
}

// NewContainerFactoryFunc returns ContainerFactory from single BOSH instance group.
type NewContainerFactoryFunc func(manifestName string, instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider bdm.ReleaseImageProvider, bpmConfigs bpm.Configs) ContainerFactory

// VolumeFactory builds Kubernetes containers from BOSH jobs.
type VolumeFactory interface {
	GenerateDefaultDisks(manifestName string, instanceGroupName *bdm.InstanceGroup, igResolvedSecretVersion string, namespace string) disk.BPMResourceDisks
	GenerateBPMDisks(manifestName string, instanceGroup *bdm.InstanceGroup, bpmConfigs bpm.Configs, namespace string) (disk.BPMResourceDisks, error)
}

// DomainNameService is a limited interface for the funcs used in the bpm converter
type DomainNameService interface {
	// HeadlessServiceName constructs the headless service name for the instance group.
	HeadlessServiceName(instanceGroupName string) string

	// DNSSetting get the DNS settings for POD.
	DNSSetting(namespace string) (corev1.DNSPolicy, *corev1.PodDNSConfig, error)
}

// NewConverter returns a new converter
func NewConverter(namespace string, volumeFactory VolumeFactory, newContainerFactoryFunc NewContainerFactoryFunc) *BPMConverter {
	return &BPMConverter{
		namespace:               namespace,
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

// Resources uses BOSH Process Manager information to create k8s container specs from single BOSH instance group.
// It returns quarks stateful sets, services and quarks jobs.
func (kc *BPMConverter) Resources(manifestName string, dns DomainNameService, qStsVersion string, instanceGroup *bdm.InstanceGroup, releaseImageProvider bdm.ReleaseImageProvider, bpmConfigs bpm.Configs, igResolvedSecretVersion string) (*Resources, error) {
	instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Set(manifestName, instanceGroup.Name, qStsVersion)

	defaultDisks := kc.volumeFactory.GenerateDefaultDisks(manifestName, instanceGroup, igResolvedSecretVersion, kc.namespace)
	bpmDisks, err := kc.volumeFactory.GenerateBPMDisks(manifestName, instanceGroup, bpmConfigs, kc.namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Generate of BPM disks failed for manifest name %s, instance group %s.", manifestName, instanceGroup.Name)
	}

	allDisks := append(defaultDisks, bpmDisks...)

	res := &Resources{
		PersistentVolumeClaims: allDisks.PVCs(),
	}

	cfac := kc.newContainerFactoryFunc(
		manifestName,
		instanceGroup.Name,
		igResolvedSecretVersion,
		instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.DisableLogSidecar,
		releaseImageProvider,
		bpmConfigs,
	)

	switch instanceGroup.LifeCycle {
	case bdm.IGTypeService, "":
		convertedExtStatefulSet, err := kc.serviceToQuarksStatefulSet(cfac, manifestName, dns, instanceGroup, defaultDisks, bpmDisks)
		if err != nil {
			return nil, err
		}

		services := kc.serviceToKubeServices(manifestName, dns, instanceGroup, &convertedExtStatefulSet)
		if len(services) != 0 {
			res.Services = append(res.Services, services...)
		}

		res.InstanceGroups = append(res.InstanceGroups, convertedExtStatefulSet)
	case bdm.IGTypeErrand, bdm.IGTypeAutoErrand:
		convertedQJob, err := kc.errandToQuarksJob(cfac, manifestName, dns, instanceGroup, defaultDisks, bpmDisks)
		if err != nil {
			return nil, err
		}

		res.Errands = append(res.Errands, convertedQJob)
	}

	return res, nil
}

// serviceToQuarksStatefulSet will generate an QuarksStatefulSet
func (kc *BPMConverter) serviceToQuarksStatefulSet(
	cfac ContainerFactory,
	manifestName string,
	dns DomainNameService,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks disk.BPMResourceDisks,
	bpmDisks disk.BPMResourceDisks,
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

	statefulSetLabels := statefulset.FilterLabels(instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels)
	statefulSetAnnotations, err := statefulset.ComputeAnnotations(instanceGroup)
	if err != nil {
		return qstsv1a1.QuarksStatefulSet{}, errors.Wrapf(err, "computing annotations failed for instance group %s", instanceGroup.Name)
	}
	extSts := qstsv1a1.QuarksStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instanceGroup.QuarksStatefulSetName(manifestName),
			Namespace:   kc.namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: qstsv1a1.QuarksStatefulSetSpec{
			Zones:                instanceGroup.AZs,
			UpdateOnConfigChange: true,
			ActivePassiveProbes:  instanceGroup.ActivePassiveProbes(),
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
							Affinity:       instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity,
							Volumes:        volumes,
							InitContainers: initContainers,
							Containers:     containers,
							SecurityContext: &corev1.PodSecurityContext{
								FSGroup: &admGroupID,
							},
							Subdomain:        dns.HeadlessServiceName(instanceGroup.Name),
							ImagePullSecrets: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ImagePullSecrets,
						},
					},
					VolumeClaimTemplates: volumeClaims,
				},
			},
		},
	}

	spec := &extSts.Spec.Template.Spec.Template.Spec
	spec.DNSPolicy, spec.DNSConfig, err = dns.DNSSetting(kc.namespace)
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
func (kc *BPMConverter) serviceToKubeServices(manifestName string, dns DomainNameService, instanceGroup *bdm.InstanceGroup, qSts *qstsv1a1.QuarksStatefulSet) []corev1.Service {
	var services []corev1.Service
	// Collect ports to be exposed for each job
	ports := instanceGroup.ServicePorts()
	if len(ports) == 0 {
		return services
	}

	activePassiveModel := false
	for _, job := range instanceGroup.Jobs {
		if len(job.Properties.Quarks.ActivePassiveProbes) > 0 {
			activePassiveModel = true
		}
	}

	if len(instanceGroup.AZs) > 0 {
		for azIndex := range instanceGroup.AZs {
			services = kc.generateServices(services,
				*instanceGroup,
				manifestName,
				azIndex,
				activePassiveModel,
				ports)
		}
	} else {
		services = kc.generateServices(services,
			*instanceGroup,
			manifestName,
			-1,
			activePassiveModel,
			ports)
	}

	headlessServiceName := dns.HeadlessServiceName(instanceGroup.Name)
	headlessServiceSelector := map[string]string{
		bdm.LabelDeploymentName:    manifestName,
		bdm.LabelInstanceGroupName: instanceGroup.Name,
	}
	if activePassiveModel {
		headlessServiceSelector[qstsv1a1.LabelActivePod] = "active"
	}
	headlessService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        headlessServiceName,
			Namespace:   kc.namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
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
	cfac ContainerFactory,
	manifestName string,
	dns DomainNameService,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks disk.BPMResourceDisks,
	bpmDisks disk.BPMResourceDisks,
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
	// Controller will delete successful job
	podLabels["delete"] = "pod"

	defaultVolumes := defaultDisks.Volumes()
	bpmVolumes := bpmDisks.Volumes()
	volumes := make([]corev1.Volume, 0, len(defaultVolumes)+len(bpmVolumes))
	volumes = append(volumes, defaultVolumes...)
	volumes = append(volumes, bpmVolumes...)

	strategy := qjv1a1.TriggerManual
	if instanceGroup.LifeCycle == bdm.IGTypeAutoErrand {
		strategy = qjv1a1.TriggerOnce
	}

	qJob := qjv1a1.QuarksJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", manifestName, instanceGroup.Name),
			Namespace:   kc.namespace,
			Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
			Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
		},
		Spec: qjv1a1.QuarksJobSpec{
			Trigger: qjv1a1.Trigger{
				Strategy: strategy,
			},
			Template: batchv1b1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:        instanceGroup.Name,
							Labels:      podLabels,
							Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
						},
						Spec: corev1.PodSpec{
							RestartPolicy:  corev1.RestartPolicyOnFailure,
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

	qJob.Spec.Template.Spec.Template.Spec.DNSPolicy, qJob.Spec.Template.Spec.Template.Spec.DNSConfig, err = dns.DNSSetting(kc.namespace)

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

func (kc *BPMConverter) generateServices(services []corev1.Service,
	instanceGroup bdm.InstanceGroup,
	manifestName string,
	azIndex int,
	activePassiveModel bool,
	ports []corev1.ServicePort) []corev1.Service {
	serviceLabels := func(azIndex, ordinal int, includeActiveSelector bool) map[string]string {
		if azIndex == -1 {
			azIndex = 0
		}
		labels := map[string]string{
			bdm.LabelDeploymentName:    manifestName,
			bdm.LabelInstanceGroupName: instanceGroup.Name,
			qstsv1a1.LabelAZIndex:      strconv.Itoa(azIndex),
			qstsv1a1.LabelPodOrdinal:   strconv.Itoa(ordinal),
		}
		if includeActiveSelector {
			labels[qstsv1a1.LabelActivePod] = "active"
		}
		return labels
	}

	for i := 0; i < instanceGroup.Instances; i++ {
		services = append(services, corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instanceGroup.IndexedServiceName(manifestName, i, azIndex),
				Namespace: kc.namespace,
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
