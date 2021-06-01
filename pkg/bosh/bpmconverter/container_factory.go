package bpmconverter

import (
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpm"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

var (
	rootUserID = int64(0)
	vcapUserID = int64(1000)
	entrypoint = []string{"/usr/bin/dumb-init", "--"}

	podOrdinalEnv = corev1.EnvVar{
		Name: EnvPodOrdinal,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.labels['quarks.cloudfoundry.org/pod-ordinal']",
			},
		},
	}

	replicasEnv = corev1.EnvVar{
		Name:  EnvReplicas,
		Value: "1",
	}
	azIndexEnv = corev1.EnvVar{
		Name:  EnvAzIndex,
		Value: "1",
	}
)

const (
	// EnvJobsDir is a key for the container Env used to lookup the jobs dir.
	EnvJobsDir = "JOBS_DIR"

	// EnvLogsDir is the path from where to tail file logs.
	EnvLogsDir = "LOGS_DIR"
)

// ContainerFactoryImpl is a concrete implementation of ContainerFactor.
type ContainerFactoryImpl struct {
	instanceGroupName    string
	errand               bool
	version              string
	disableLogSidecar    bool
	releaseImageProvider bdm.ReleaseImageProvider
	bpmConfigs           bpm.Configs
}

// NewContainerFactory returns a concrete implementation of ContainerFactory.
func NewContainerFactory(igName string, errand bool, version string, disableLogSidecar bool, releaseImageProvider bdm.ReleaseImageProvider, bpmConfigs bpm.Configs) ContainerFactory {
	return &ContainerFactoryImpl{
		instanceGroupName:    igName,
		errand:               errand,
		version:              version,
		disableLogSidecar:    disableLogSidecar,
		releaseImageProvider: releaseImageProvider,
		bpmConfigs:           bpmConfigs,
	}
}

// JobSpecCopierContainer will return a corev1.Container{} with the populated field.
func JobSpecCopierContainer(releaseName string, jobImage string, volumeMountName string) corev1.Container {
	inContainerReleasePath := filepath.Join(VolumeRenderingDataMountPath, "jobs-src", releaseName)
	return corev1.Container{
		Name:  names.Sanitize(fmt.Sprintf("spec-copier-%s", releaseName)),
		Image: jobImage,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeMountName,
				MountPath: VolumeRenderingDataMountPath,
			},
		},
		Command: entrypoint,
		Args: []string{
			"/bin/sh",
			"-xc",
			fmt.Sprintf("time sh -c 'mkdir -p %s && cp -ar %s/* %s && chown vcap:vcap %s -R'", inContainerReleasePath, VolumeJobsSrcDirMountPath, inContainerReleasePath, inContainerReleasePath),
		},
	}
}

// capability converts string slice into Capability slice of kubernetes.
func capability(s []string) []corev1.Capability {
	capabilities := make([]corev1.Capability, len(s))
	for capabilityIndex, capability := range s {
		capabilities[capabilityIndex] = corev1.Capability(capability)
	}
	return capabilities
}

// proccessVolumentMounts returns the volumes for a process in a special order
func proccessVolumentMounts(defaultVolumeMounts []corev1.VolumeMount, processDisks bdm.Disks, ephemeralMount *corev1.VolumeMount, persistentDiskMount *corev1.VolumeMount) []corev1.VolumeMount {
	bpmVolumeMounts := make([]corev1.VolumeMount, 0)
	for _, processDisk := range processDisks {
		bpmVolumeMounts = append(bpmVolumeMounts, *processDisk.VolumeMount)
	}

	v := append(defaultVolumeMounts, bpmVolumeMounts...)

	if ephemeralMount != nil {
		v = append(v, *ephemeralMount)
	}
	if persistentDiskMount != nil {
		v = append(v, *persistentDiskMount)
	}

	return v
}
