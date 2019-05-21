package manifest

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

// ContainerFactory builds Kubernetes containers from BOSH jobs
type ContainerFactory struct {
	manifestName         string
	igName               string
	releaseImageProvider releaseImageProvider
	bpmConfigs           bpm.Configs
}

// NewContainerFactory returns a new ContainerFactory for a BOSH instant group
func NewContainerFactory(manifestName string, igName string, releaseImageProvider releaseImageProvider, bpmConfigs bpm.Configs) *ContainerFactory {
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
			MountPath: fmt.Sprintf("/var/run/secrets/resolved-properties/%s", c.igName),
			ReadOnly:  true,
		},
	}

	initContainers = append(initContainers, corev1.Container{
		Name:         fmt.Sprintf("renderer-%s", c.igName),
		Image:        GetOperatorDockerImage(),
		VolumeMounts: volumeMounts,
		Env: []corev1.EnvVar{
			{
				Name:  "INSTANCE_GROUP_NAME",
				Value: c.igName,
			},
			{
				Name:  "BOSH_MANIFEST_PATH",
				Value: fmt.Sprintf("/var/run/secrets/resolved-properties/%s/properties.yaml", c.igName),
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

// JobsToContainers creates a list of Containers for corev1.PodSpec Containers field
func (c *ContainerFactory) JobsToContainers(jobs []Job) ([]corev1.Container, error) {
	generateJobContainers := func(job Job, jobImage string) (error, []corev1.Container) {
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
			},
		}

		bpmConfig, ok := c.bpmConfigs[boshJobName]
		if !ok {
			return errors.Errorf("failed to lookup bpm config for bosh job '%s' in bpm configs", boshJobName), containers
		}

		if len(bpmConfig.Processes) < 1 {
			return errors.New("bpm info has no processes"), containers
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

		return nil, containers
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

		err, processes := generateJobContainers(job, jobImage)
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
