package manifest

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

const (
	// EnvJobsDir is a key for the container Env used to lookup the jobs dir
	EnvJobsDir = "JOBS_DIR"
)

// ContainerFactory builds Kubernetes containers from BOSH jobs
type ContainerFactory struct {
	manifestName         string
	igName               string
	releaseImageProvider ReleaseImageProvider
	bpmConfigs           bpm.Configs
}

// NewContainerFactory returns a new ContainerFactory for a BOSH instant group
func NewContainerFactory(manifestName string, igName string, releaseImageProvider ReleaseImageProvider, bpmConfigs bpm.Configs) *ContainerFactory {
	return &ContainerFactory{
		manifestName:         manifestName,
		igName:               igName,
		releaseImageProvider: releaseImageProvider,
		bpmConfigs:           bpmConfigs,
	}
}

// JobsToInitContainers creates a list of Containers for corev1.PodSpec InitContainers field
func (c *ContainerFactory) JobsToInitContainers(jobs []Job, hasPersistentDisk bool) ([]corev1.Container, error) {
	copyingSpecsInitContainers := make([]corev1.Container, 0)
	boshPreStartInitContainers := make([]corev1.Container, 0)
	bpmPreStartInitContainers := make([]corev1.Container, 0)

	copyingSpecsUniq := map[string]struct{}{}
	for _, job := range jobs {
		jobImage, err := c.releaseImageProvider.GetReleaseImage(c.igName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}

		// One copying specs init container for each release.
		if _, done := copyingSpecsUniq[job.Release]; !done {
			copyingSpecsUniq[job.Release] = struct{}{}
			copyingSpecsInitContainer := jobSpecCopierContainer(job.Release, jobImage, VolumeRenderingDataName)
			copyingSpecsInitContainers = append(copyingSpecsInitContainers, copyingSpecsInitContainer)
		}

		// Setup the BOSH pre-start init containers.
		boshPreStartInitContainers = append(boshPreStartInitContainers, boshPreStartInitContainer(job.Name, jobImage, hasPersistentDisk))

		// Setup the BPM pre-start init containers.
		bpmConfig, ok := c.bpmConfigs[job.Name]
		if !ok {
			return []corev1.Container{}, errors.Errorf("failed to lookup bpm config for bosh job '%s' in bpm configs", job.Name)
		}
		for _, process := range bpmConfig.Processes {
			if process.Hooks.PreStart != "" {
				container := bpmPrestartInitContainer(process, jobImage)

				bpmVolumes, err := generateBPMVolumes(process, job.Name)
				if err != nil {
					return []corev1.Container{}, err
				}
				container.VolumeMounts = append(instanceGroupVolumeMounts(), bpmVolumes...)

				bpmPreStartInitContainers = append(bpmPreStartInitContainers, container)
			}
		}
	}

	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		c.manifestName,
		c.igName,
		true,
	)

	initContainers := flattenContainers(
		copyingSpecsInitContainers,
		templateRenderingContainer(c.igName, resolvedPropertiesSecretName),
		createDirContainer(c.igName, jobs),
		boshPreStartInitContainers,
		bpmPreStartInitContainers,
	)

	return initContainers, nil
}

// JobsToContainers creates a list of Containers for corev1.PodSpec Containers field
func (c *ContainerFactory) JobsToContainers(jobs []Job) ([]corev1.Container, error) {
	var containers []corev1.Container

	if len(jobs) == 0 {
		return nil, fmt.Errorf("instance group %s has no jobs defined", c.igName)
	}

	for _, job := range jobs {
		jobImage, err := c.releaseImageProvider.GetReleaseImage(c.igName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}

		bpmConfig, ok := c.bpmConfigs[job.Name]
		if !ok {
			return nil, errors.Errorf("failed to lookup bpm config for bosh job '%s' in bpm configs", job.Name)
		}

		if len(bpmConfig.Processes) < 1 {
			return nil, errors.New("bpm info has no processes")
		}

		for _, process := range bpmConfig.Processes {
			container := bpmProcessContainer(
				fmt.Sprintf("%s-%s", job.Name, process.Name),
				jobImage,
				process,
				job.Properties.BOSHContainerization.Run.HealthChecks,
			)

			bpmVolumes, err := generateBPMVolumes(process, job.Name)
			if err != nil {
				return []corev1.Container{}, err
			}
			container.VolumeMounts = append(instanceGroupVolumeMounts(), bpmVolumes...)

			containers = append(containers, container)
		}
	}
	return containers, nil
}

// generateBPMVolumes defines any other volumes required to be mounted,
// based on the bpm process schema definition. This looks for:
// - ephemeral_disk (boolean)
// - persistent_disk (boolean)
// - additional_volumes (list of volumes)
func generateBPMVolumes(bpmProcess bpm.Process, jobName string) ([]corev1.VolumeMount, error) {
	var bpmVolumes []corev1.VolumeMount

	r, _ := regexp.Compile(AdditionalVolumesRegex)
	rS, _ := regexp.Compile(AdditionalVolumesVcapStoreRegex)
	// Process all ephemeral_disk
	if bpmProcess.EphemeralDisk {
		bpmEphemeralDisk := corev1.VolumeMount{
			Name:      fmt.Sprintf("%s-%s", VolumeEphemeralDirName, jobName),
			MountPath: fmt.Sprintf("%s%s", VolumeEphemeralDirMountPath, jobName),
		}
		bpmVolumes = append(bpmVolumes, bpmEphemeralDisk)
	}

	// TODO: skip this, while we need to figure it out a better way
	// to define persistenVolumeClaims for jobs
	// if bpmProcess.PersistentDisk {
	// }

	// Process all additional_volumes
	for i, additionalVolume := range bpmProcess.AdditionalVolumes {
		match := r.MatchString(additionalVolume.Path)
		if !match {
			return []corev1.VolumeMount{}, errors.Errorf("The %s path, must be a path inside"+
				" /var/vcap/data, /var/vcap/store or /var/vcap/sys/run, for a path outside these,"+
				" you must use the unrestricted_volumes key", additionalVolume.Path)
		}

		matchVcapStore := rS.MatchString(additionalVolume.Path)
		// TODO: skip additional volumes under /var/vcap/store
		// while we need to figure it out a better way to define
		// persistenVolumeClaims for jobs
		if matchVcapStore {
			continue
		}
		// TODO: How to map the following bpm volume schema fields
		// - allow_executions
		// - mount_only
		// into a corev1.VolumeMount
		bpmAdditionalVolume := corev1.VolumeMount{
			Name:      fmt.Sprintf("%s-%s-%b", AdditionalVolume, jobName, i),
			ReadOnly:  !additionalVolume.Writable,
			MountPath: additionalVolume.Path,
		}
		bpmVolumes = append(bpmVolumes, bpmAdditionalVolume)
	}

	// Process the unsafe configuration
	for i, unrestrictedVolumes := range bpmProcess.Unsafe.UnrestrictedVolumes {
		matchVcapStore := rS.MatchString(unrestrictedVolumes.Path)

		// TODO: skip unrestricted volumes under /var/vcap/store
		// while we need to figure it out a better way to define
		// persistenVolumeClaims for jobs
		if matchVcapStore {
			continue
		}
		uVolume := corev1.VolumeMount{
			Name:      fmt.Sprintf("%s-%s-%b", UnrestrictedVolume, jobName, i),
			ReadOnly:  !unrestrictedVolumes.Writable,
			MountPath: unrestrictedVolumes.Path,
		}
		bpmVolumes = append(bpmVolumes, uVolume)

	}

	return bpmVolumes, nil
}

// jobSpecCopierContainer will return a corev1.Container{} with the populated field
func jobSpecCopierContainer(releaseName string, jobImage string, volumeMountName string) corev1.Container {
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
		Command: []string{
			"bash",
			"-c",
			fmt.Sprintf(`mkdir -p "%s" && cp -ar %s/* "%s"`, inContainerReleasePath, VolumeJobsSrcDirMountPath, inContainerReleasePath),
		},
	}
}

func templateRenderingContainer(igName string, secretName string) corev1.Container {
	return corev1.Container{
		Name:  names.Sanitize(fmt.Sprintf("renderer-%s", igName)),
		Image: GetOperatorDockerImage(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      VolumeRenderingDataName,
				MountPath: VolumeRenderingDataMountPath,
			},
			{
				Name:      VolumeJobsDirName,
				MountPath: VolumeJobsDirMountPath,
			},
			{
				Name:      generateVolumeName(secretName),
				MountPath: fmt.Sprintf("/var/run/secrets/resolved-properties/%s", igName),
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  EnvInstanceGroupName,
				Value: igName,
			},
			{
				Name:  EnvBOSHManifestPath,
				Value: fmt.Sprintf("/var/run/secrets/resolved-properties/%s/properties.yaml", igName),
			},
			{
				Name:  EnvJobsDir,
				Value: VolumeRenderingDataMountPath,
			},
		},
		Args: []string{"util", "template-render"},
	}
}

func createDirContainer(name string, jobs []Job) corev1.Container {
	dirs := []string{}
	for _, job := range jobs {
		jobDirs := append(job.dataDirs(job.Name), job.sysDirs(job.Name)...)
		dirs = append(dirs, jobDirs...)
	}

	return corev1.Container{
		Name:  names.Sanitize(fmt.Sprintf("create-dirs-%s", name)),
		Image: GetOperatorDockerImage(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      VolumeDataDirName,
				MountPath: VolumeDataDirMountPath,
			},
			{
				Name:      VolumeSysDirName,
				MountPath: VolumeSysDirMountPath,
			},
		},
		Env:     []corev1.EnvVar{},
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " "))},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: &vcapUserID,
		},
	}
}

func boshPreStartInitContainer(jobName string, jobImage string, hasPersistentDisk bool) corev1.Container {
	boshPreStart := filepath.Join(VolumeJobsDirMountPath, jobName, "bin", "pre-start")
	c := corev1.Container{
		Name:         names.Sanitize(fmt.Sprintf("bosh-pre-start-%s", jobName)),
		Image:        jobImage,
		VolumeMounts: instanceGroupVolumeMounts(),
		Command:      []string{"/bin/sh", "-c"},
		Args:         []string{fmt.Sprintf(`if [ -x "%[1]s" ]; then "%[1]s"; fi`, boshPreStart)},
	}
	if hasPersistentDisk {
		persistentDisk := corev1.VolumeMount{
			Name:      VolumeStoreDirName,
			MountPath: VolumeStoreDirMountPath,
		}
		c.VolumeMounts = append(c.VolumeMounts, persistentDisk)
	}
	return c
}

func bpmPrestartInitContainer(process bpm.Process, jobImage string) corev1.Container {
	return corev1.Container{
		Name:    names.Sanitize(fmt.Sprintf("bpm-pre-start-%s", process.Name)),
		Image:   jobImage,
		Command: []string{process.Hooks.PreStart},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &process.Unsafe.Privileged,
		},
	}
}

func bpmProcessContainer(name string, jobImage string, process bpm.Process, healthchecks map[string]HealthCheck) corev1.Container {
	container := corev1.Container{
		Name:       names.Sanitize(name),
		Image:      jobImage,
		Command:    []string{process.Executable},
		Args:       process.Args,
		WorkingDir: process.Workdir,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: capability(process.Capabilities),
			},
			Privileged: &process.Unsafe.Privileged,
		},
	}

	for name, value := range process.Env {
		container.Env = append(container.Env, corev1.EnvVar{Name: name, Value: value})
	}

	for name, hc := range healthchecks {
		if name == process.Name {
			if hc.ReadinessProbe != nil {
				container.ReadinessProbe = hc.ReadinessProbe
			}
			if hc.LivenessProbe != nil {
				container.LivenessProbe = hc.LivenessProbe
			}
		}
	}
	return container
}

// capability converts string slice into Capability slice of kubernetes
func capability(s []string) []corev1.Capability {
	capabilities := make([]corev1.Capability, len(s))
	for capabilityIndex, capability := range s {
		capabilities[capabilityIndex] = corev1.Capability(capability)
	}
	return capabilities
}

func instanceGroupVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      VolumeRenderingDataName,
			MountPath: VolumeRenderingDataMountPath,
		},
		{
			Name:      VolumeJobsDirName,
			MountPath: VolumeJobsDirMountPath,
		},
		{
			Name:      VolumeDataDirName,
			MountPath: VolumeDataDirMountPath,
		},
		{
			Name:      VolumeSysDirName,
			MountPath: VolumeSysDirMountPath,
		},
	}
}

// flattenContainers will flatten the containers parameter. Each argument passed to
// flattenContainers should be a corev1.Container or []corev1.Container. The final
// []corev1.Container creation is optimized to prevent slice re-allocation.
func flattenContainers(containers ...interface{}) []corev1.Container {
	var totalLen int
	for _, instance := range containers {
		switch v := instance.(type) {
		case []corev1.Container:
			totalLen += len(v)
		case corev1.Container:
			totalLen++
		}
	}
	result := make([]corev1.Container, 0, totalLen)
	for _, instance := range containers {
		switch v := instance.(type) {
		case []corev1.Container:
			result = append(result, v...)
		case corev1.Container:
			result = append(result, v)
		}
	}
	return result
}
