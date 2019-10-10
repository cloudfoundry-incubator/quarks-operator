package converter

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1b1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/disk"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	ejv1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/pointers"
)

var (
	admGroupID = int64(1000)
)

// ReleaseImageProvider interface to provide the docker release image for a BOSH job
// This lookup is currently implemented by the manifest model.
type ReleaseImageProvider interface {
	// GetReleaseImage returns the release image for an job in an instance group
	GetReleaseImage(instanceGroupName, jobName string) (string, error)
}

// BPMResources contains BPM related k8s resources, which were converted from BOSH objects
type BPMResources struct {
	InstanceGroups         []essv1.ExtendedStatefulSet
	Errands                []ejv1.ExtendedJob
	Services               []corev1.Service
	PersistentVolumeClaims []corev1.PersistentVolumeClaim
}

// BPMResources uses BOSH Process Manager information to create k8s container specs from single BOSH instance group.
// It returns extended stateful sets, services and extended jobs.
func (kc *KubeConverter) BPMResources(manifestName string, dns manifest.DomainNameService, version string, instanceGroup *bdm.InstanceGroup, releaseImageProvider ReleaseImageProvider, bpmConfigs bpm.Configs) (*BPMResources, error) {
	instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Set(manifestName, instanceGroup.Name, version)

	defaultDisks := kc.volumeFactory.GenerateDefaultDisks(manifestName, instanceGroup.Name, version, kc.namespace)
	bpmDisks, err := kc.volumeFactory.GenerateBPMDisks(manifestName, instanceGroup, bpmConfigs, kc.namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "Generate of BPM disks failed for manifest name %s, instance group %s.", manifestName, instanceGroup.Name)
	}

	allDisks := append(defaultDisks, bpmDisks...)

	res := &BPMResources{
		PersistentVolumeClaims: allDisks.PVCs(),
	}

	cfac := kc.newContainerFactoryFunc(
		manifestName,
		instanceGroup.Name,
		version,
		instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.DisableLogSidecar,
		releaseImageProvider,
		bpmConfigs,
	)

	switch instanceGroup.LifeCycle {
	case bdm.IGTypeService, "":
		convertedExtStatefulSet, err := kc.serviceToExtendedSts(cfac, manifestName, dns, instanceGroup, defaultDisks, bpmDisks)
		if err != nil {
			return nil, err
		}

		services := kc.serviceToKubeServices(manifestName, dns, instanceGroup, &convertedExtStatefulSet)
		if len(services) != 0 {
			res.Services = append(res.Services, services...)
		}

		res.InstanceGroups = append(res.InstanceGroups, convertedExtStatefulSet)
	case bdm.IGTypeErrand, bdm.IGTypeAutoErrand:
		convertedEJob, err := kc.errandToExtendedJob(cfac, manifestName, dns, instanceGroup, defaultDisks, bpmDisks)
		if err != nil {
			return nil, err
		}

		res.Errands = append(res.Errands, convertedEJob)
	}

	return res, nil
}

// serviceToExtendedSts will generate an ExtendedStatefulSet
func (kc *KubeConverter) serviceToExtendedSts(
	cfac ContainerFactory,
	manifestName string,
	dns manifest.DomainNameService,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks disk.BPMResourceDisks,
	bpmDisks disk.BPMResourceDisks,
) (essv1.ExtendedStatefulSet, error) {
	defaultVolumeMounts := defaultDisks.VolumeMounts()
	initContainers, err := cfac.JobsToInitContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks, instanceGroup.Properties.Quarks.RequiredService)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, errors.Wrapf(err, "building initContainers failed for instance group %s", instanceGroup.Name)
	}

	containers, err := cfac.JobsToContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, errors.Wrapf(err, "building containers failed for instance group %s", instanceGroup.Name)
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
			Zones:                instanceGroup.AZs,
			UpdateOnConfigChange: true,
			Template: v1beta2.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:        instanceGroup.NameSanitized(),
					Labels:      instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels,
					Annotations: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Annotations,
				},
				Spec: v1beta2.StatefulSetSpec{
					Replicas: pointers.Int32(int32(instanceGroup.Instances)),
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
							Subdomain:        dns.HeadlessServiceName(instanceGroup.Name),
							ImagePullSecrets: instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ImagePullSecrets,
						},
					},
				},
			},
		},
	}

	spec := &extSts.Spec.Template.Spec.Template.Spec
	spec.DNSPolicy, spec.DNSConfig, err = dns.DNSSetting(kc.namespace)
	if err != nil {
		return essv1.ExtendedStatefulSet{}, err
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
func (kc *KubeConverter) serviceToKubeServices(manifestName string, dns manifest.DomainNameService, instanceGroup *bdm.InstanceGroup, eSts *essv1.ExtendedStatefulSet) []corev1.Service {
	var services []corev1.Service
	// Collect ports to be exposed for each job
	ports := instanceGroup.ServicePorts()
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

	headlessServiceName := dns.HeadlessServiceName(instanceGroup.Name)
	headlessService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        headlessServiceName,
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

	// Set headlessService to govern StatefulSet.
	eSts.Spec.Template.Spec.ServiceName = headlessServiceName

	services = append(services, headlessService)

	return services
}

// errandToExtendedJob will generate an ExtendedJob
func (kc *KubeConverter) errandToExtendedJob(
	cfac ContainerFactory,
	manifestName string,
	dns manifest.DomainNameService,
	instanceGroup *bdm.InstanceGroup,
	defaultDisks disk.BPMResourceDisks,
	bpmDisks disk.BPMResourceDisks,
) (ejv1.ExtendedJob, error) {
	defaultVolumeMounts := defaultDisks.VolumeMounts()
	initContainers, err := cfac.JobsToInitContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks, instanceGroup.Properties.Quarks.RequiredService)
	if err != nil {
		return ejv1.ExtendedJob{}, errors.Wrapf(err, "building initContainers failed for instance group %s", instanceGroup.Name)
	}

	containers, err := cfac.JobsToContainers(instanceGroup.Jobs, defaultVolumeMounts, bpmDisks)
	if err != nil {
		return ejv1.ExtendedJob{}, errors.Wrapf(err, "building containers failed for instance group %s", instanceGroup.Name)
	}

	podLabels := instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Labels
	// Controller will delete successful job
	podLabels["delete"] = "pod"

	defaultVolumes := defaultDisks.Volumes()
	bpmVolumes := bpmDisks.Volumes()
	volumes := make([]corev1.Volume, 0, len(defaultVolumes)+len(bpmVolumes))
	volumes = append(volumes, defaultVolumes...)
	volumes = append(volumes, bpmVolumes...)

	strategy := ejv1.TriggerManual
	if instanceGroup.LifeCycle == bdm.IGTypeAutoErrand {
		strategy = ejv1.TriggerOnce
	}

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

	eJob.Spec.Template.Spec.Template.Spec.DNSPolicy, eJob.Spec.Template.Spec.Template.Spec.DNSConfig, err = dns.DNSSetting(kc.namespace)

	if err != nil {
		return ejv1.ExtendedJob{}, err
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity != nil {
		eJob.Spec.Template.Spec.Template.Spec.Affinity = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.Affinity
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName != "" {
		eJob.Spec.Template.Spec.Template.Spec.ServiceAccountName = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.ServiceAccountName
	}

	if instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken != nil {
		eJob.Spec.Template.Spec.Template.Spec.AutomountServiceAccountToken = instanceGroup.Env.AgentEnvBoshConfig.Agent.Settings.AutomountServiceAccountToken
	}

	return eJob, nil
}
