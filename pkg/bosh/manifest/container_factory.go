package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
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
func (c *ContainerFactory) JobsToInitContainers(jobs []Job) ([]corev1.Container, error) {
	initContainers := []corev1.Container{}

	// one init container for each release, for copying specs
	doneReleases := map[string]bool{}
	for _, job := range jobs {
		if _, ok := doneReleases[job.Release]; ok {
			continue
		}

		doneReleases[job.Release] = true
		releaseImage, err := c.releaseImageProvider.GetReleaseImage(c.igName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}
		initContainers = append(initContainers, jobSpecCopierContainer(job.Release, releaseImage, "rendering-data"))

	}

	_, resolvedPropertiesSecretName := names.CalculateEJobOutputSecretPrefixAndName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties,
		c.manifestName,
		c.igName,
		true,
	)
	initContainers = append(initContainers, templateRenderingContainer(c.igName, resolvedPropertiesSecretName))
	initContainers = append(initContainers, createDirContainer(c.igName, jobs))

	return initContainers, nil
}

// JobsToContainers creates a list of Containers for corev1.PodSpec Containers field
func (c *ContainerFactory) JobsToContainers(jobs []Job) ([]corev1.Container, error) {
	generateJobContainers := func(job Job, jobImage string) ([]corev1.Container, error) {
		boshJobName := job.Name
		containers := []corev1.Container{}
		template := corev1.Container{
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
				{
					Name:      "data-dir",
					MountPath: "/var/vcap/data",
				},
				{
					Name:      "sys-dir",
					MountPath: "/var/vcap/sys",
				},
			},
		}

		bpmConfig, ok := c.bpmConfigs[boshJobName]
		if !ok {
			return containers, errors.Errorf("failed to lookup bpm config for bosh job '%s' in bpm configs", boshJobName)
		}

		if len(bpmConfig.Processes) < 1 {
			return containers, errors.New("bpm info has no processes")
		}

		for _, process := range bpmConfig.Processes {
			c := template.DeepCopy()

			c.Name = fmt.Sprintf("%s-%s", boshJobName, process.Name)
			c.Command = []string{process.Executable}
			c.Args = process.Args
			for name, value := range process.Env {
				c.Env = append(template.Env, corev1.EnvVar{Name: name, Value: value})
			}
			c.WorkingDir = process.Workdir
			c.SecurityContext = &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: ToCapability(process.Capabilities),
				},
			}

			if len(job.Properties.BOSHContainerization.Run.HealthChecks) > 0 {
				for name, hc := range job.Properties.BOSHContainerization.Run.HealthChecks {
					if name == process.Name {
						if hc.ReadinessProbe != nil {
							c.ReadinessProbe = hc.ReadinessProbe
						}
						if hc.LivenessProbe != nil {
							c.LivenessProbe = hc.LivenessProbe
						}
					}
				}
			}

			containers = append(containers, *c)
		}

		return containers, nil
	}

	var containers []corev1.Container

	if len(jobs) == 0 {
		return nil, fmt.Errorf("instance group %s has no jobs defined", c.igName)
	}

	for _, job := range jobs {
		jobImage, err := c.releaseImageProvider.GetReleaseImage(c.igName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}

		processes, err := generateJobContainers(job, jobImage)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to apply bpm information on bosh job '%s', instance group '%s'", job.Name, c.igName)
		}

		containers = append(containers, processes...)
	}
	return containers, nil
}

// jobSpecCopierContainer will return a corev1.Container{} with the populated field
func jobSpecCopierContainer(releaseName string, releaseImage string, volumeMountName string) corev1.Container {
	inContainerReleasePath := filepath.Join("/var/vcap/all-releases/jobs-src", releaseName)
	return corev1.Container{
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
}

func templateRenderingContainer(name string, secretName string) corev1.Container {
	return corev1.Container{
		Name:  fmt.Sprintf("renderer-%s", name),
		Image: GetOperatorDockerImage(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "rendering-data",
				MountPath: "/var/vcap/all-releases",
			},
			{
				Name:      "jobs-dir",
				MountPath: "/var/vcap/jobs",
			},
			{
				Name:      generateVolumeName(secretName),
				MountPath: fmt.Sprintf("/var/run/secrets/resolved-properties/%s", name),
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "INSTANCE_GROUP_NAME",
				Value: name,
			},
			{
				Name:  "BOSH_MANIFEST_PATH",
				Value: fmt.Sprintf("/var/run/secrets/resolved-properties/%s/properties.yaml", name),
			},
			{
				Name:  "JOBS_DIR",
				Value: "/var/vcap/all-releases",
			},
		},
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", `cf-operator util template-render`},
	}
}

func createDirContainer(name string, jobs []Job) corev1.Container {
	dirs := []string{}
	for _, job := range jobs {
		jobDirs := append(job.dataDirs(job.Name), job.sysDirs(job.Name)...)
		dirs = append(dirs, jobDirs...)
	}

	return corev1.Container{
		Name:  fmt.Sprintf("create-dirs-%s", name),
		Image: GetOperatorDockerImage(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "data-dir",
				MountPath: "/var/vcap/data",
			},
			{
				Name:      "sys-dir",
				MountPath: "/var/vcap/sys",
			},
		},
		Env:     []corev1.EnvVar{},
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", "mkdir -p " + strings.Join(dirs, " ")},
	}
}

// ToCapability converts string slice into Capability slice of kubernetes
func ToCapability(s []string) []corev1.Capability {
	capabilities := make([]corev1.Capability, len(s))
	for capabilityIndex, capability := range s {
		capabilities[capabilityIndex] = corev1.Capability(capability)
	}
	return capabilities
}
