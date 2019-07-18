package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	bc "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest/containerization"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

const (
	// EnvJobsDir is a key for the container Env used to lookup the jobs dir
	EnvJobsDir = "JOBS_DIR"
)

// ContainerFactory builds Kubernetes containers from BOSH jobs
type ContainerFactory struct {
	manifestName         string
	instanceGroupName    string
	version              string
	releaseImageProvider ReleaseImageProvider
	bpmConfigs           bpm.Configs
}

// NewContainerFactory returns a new ContainerFactory for a BOSH instant group
func NewContainerFactory(manifestName string, instanceGroupName string, version string, releaseImageProvider ReleaseImageProvider, bpmConfigs bpm.Configs) *ContainerFactory {
	return &ContainerFactory{
		manifestName:         manifestName,
		instanceGroupName:    instanceGroupName,
		version:              version,
		releaseImageProvider: releaseImageProvider,
		bpmConfigs:           bpmConfigs,
	}
}

// JobsToInitContainers creates a list of Containers for corev1.PodSpec InitContainers field
func (c *ContainerFactory) JobsToInitContainers(
	jobs []Job,
	defaultVolumeMounts []corev1.VolumeMount,
	bpmDisks BPMResourceDisks,
) ([]corev1.Container, error) {
	copyingSpecsInitContainers := make([]corev1.Container, 0)
	boshPreStartInitContainers := make([]corev1.Container, 0)
	bpmPreStartInitContainers := make([]corev1.Container, 0)

	copyingSpecsUniq := map[string]struct{}{}
	for _, job := range jobs {
		jobImage, err := c.releaseImageProvider.GetReleaseImage(c.instanceGroupName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}

		// One copying specs init container for each release.
		if _, done := copyingSpecsUniq[job.Release]; !done {
			copyingSpecsUniq[job.Release] = struct{}{}
			copyingSpecsInitContainer := jobSpecCopierContainer(job.Release, jobImage, VolumeRenderingDataName)
			copyingSpecsInitContainers = append(copyingSpecsInitContainers, copyingSpecsInitContainer)
		}

		// Setup the BPM pre-start init containers before the BOSH pre-start init container in order to
		// collect all the extra BPM volumes and pass them to the BOSH pre-start init container.
		bpmConfig, ok := c.bpmConfigs[job.Name]
		if !ok {
			return []corev1.Container{}, errors.Errorf("failed to lookup bpm config for bosh job '%s' in bpm configs", job.Name)
		}

		jobDisks := bpmDisks.Filter("job_name", job.Name)
		var ephemeralMount *corev1.VolumeMount
		ephemeralDisks := jobDisks.Filter("ephemeral", "true")
		if len(ephemeralDisks) > 0 {
			ephemeralMount = ephemeralDisks[0].VolumeMount
		}
		var persistentDiskMount *corev1.VolumeMount
		persistentDiskDisks := jobDisks.Filter("persistent", "true")
		if len(persistentDiskDisks) > 0 {
			persistentDiskMount = persistentDiskDisks[0].VolumeMount
		}

		for _, process := range bpmConfig.Processes {
			if process.Hooks.PreStart != "" {
				processDisks := jobDisks.Filter("process_name", process.Name)
				bpmVolumeMounts := make([]corev1.VolumeMount, 0)
				for _, processDisk := range processDisks {
					bpmVolumeMounts = append(bpmVolumeMounts, *processDisk.VolumeMount)
				}
				processVolumeMounts := append(defaultVolumeMounts, bpmVolumeMounts...)
				if ephemeralMount != nil {
					processVolumeMounts = append(processVolumeMounts, *ephemeralMount)
				}
				if persistentDiskMount != nil {
					processVolumeMounts = append(processVolumeMounts, *persistentDiskMount)
				}
				container := bpmPreStartInitContainer(
					process,
					jobImage,
					processVolumeMounts,
					job.Properties.BOSHContainerization.Debug,
				)

				bpmPreStartInitContainers = append(bpmPreStartInitContainers, *container.DeepCopy())
			}
		}

		// Setup the BOSH pre-start init container for the job.
		boshPreStartInitContainer := boshPreStartInitContainer(
			job.Name,
			jobImage,
			append(defaultVolumeMounts, bpmDisks.VolumeMounts()...),
			job.Properties.BOSHContainerization.Debug,
		)
		boshPreStartInitContainers = append(boshPreStartInitContainers, *boshPreStartInitContainer.DeepCopy())
	}

	resolvedPropertiesSecretName := names.CalculateIGSecretName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties, // ig-resolved
		c.manifestName,
		c.instanceGroupName,
		c.version,
	)

	initContainers := flattenContainers(
		copyingSpecsInitContainers,
		templateRenderingContainer(c.instanceGroupName, resolvedPropertiesSecretName),
		createDirContainer(jobs),
		boshPreStartInitContainers,
		bpmPreStartInitContainers,
	)

	return initContainers, nil
}

// JobsToContainers creates a list of Containers for corev1.PodSpec Containers field
func (c *ContainerFactory) JobsToContainers(
	jobs []Job,
	defaultVolumeMounts []corev1.VolumeMount,
	bpmDisks BPMResourceDisks,
) ([]corev1.Container, error) {
	var containers []corev1.Container

	if len(jobs) == 0 {
		return nil, fmt.Errorf("instance group %s has no jobs defined", c.instanceGroupName)
	}

	for _, job := range jobs {
		jobImage, err := c.releaseImageProvider.GetReleaseImage(c.instanceGroupName, job.Name)
		if err != nil {
			return []corev1.Container{}, err
		}

		bpmConfig, ok := c.bpmConfigs[job.Name]
		if !ok {
			return nil, errors.Errorf("failed to lookup bpm config for bosh job '%s' in bpm configs", job.Name)
		}

		jobDisks := bpmDisks.Filter("job_name", job.Name)
		var ephemeralMount *corev1.VolumeMount
		ephemeralDisks := jobDisks.Filter("ephemeral", "true")
		if len(ephemeralDisks) > 0 {
			ephemeralMount = ephemeralDisks[0].VolumeMount
		}
		var persistentDiskMount *corev1.VolumeMount
		persistentDiskDisks := jobDisks.Filter("persistent", "true")
		if len(persistentDiskDisks) > 0 {
			persistentDiskMount = persistentDiskDisks[0].VolumeMount
		}

		for _, process := range bpmConfig.Processes {
			processDisks := jobDisks.Filter("process_name", process.Name)
			bpmVolumeMounts := make([]corev1.VolumeMount, 0)
			for _, processDisk := range processDisks {
				bpmVolumeMounts = append(bpmVolumeMounts, *processDisk.VolumeMount)
			}
			processVolumeMounts := append(defaultVolumeMounts, bpmVolumeMounts...)
			if ephemeralMount != nil {
				processVolumeMounts = append(processVolumeMounts, *ephemeralMount)
			}
			if persistentDiskMount != nil {
				processVolumeMounts = append(processVolumeMounts, *persistentDiskMount)
			}

			container := bpmProcessContainer(
				job.Name,
				process.Name,
				jobImage,
				process,
				processVolumeMounts,
				job.Properties.BOSHContainerization.Run.HealthChecks,
			)

			containers = append(containers, *container.DeepCopy())
		}
	}

	logsTailer := logsTailerContainer(c.instanceGroupName)
	containers = append(containers, logsTailer)

	return containers, nil
}

// logsTailerContainer is a container that tails all logs in /var/vcap/sys/log
func logsTailerContainer(instanceGroupName string) corev1.Container {

	rootUserID := int64(0)

	return corev1.Container{
		Name:  "logs",
		Image: GetOperatorDockerImage(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      VolumeSysDirName,
				MountPath: VolumeSysDirMountPath,
			},
		},
		Command: []string{
			"/bin/sh",
		},
		Args: []string{
			"-c",
			`
MONITORDIR="/var/vcap/sys/log/"
find "${MONITORDIR}" -type f -exec tail -n 0 -f "$file" {} + &
inotifywait -m -r -e create --format '%w%f' "${MONITORDIR}" | while read NEWFILE
do
  pkill tail
  find "${MONITORDIR}" -type f -exec tail -n 0 -f "$file" {} + &
done
`,
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: &rootUserID,
		},
	}
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
			"/bin/sh",
		},
		Args: []string{
			"-xc",
			fmt.Sprintf("mkdir -p %s && cp -ar %s/* %s && chown vcap:vcap %s -R", inContainerReleasePath, VolumeJobsSrcDirMountPath, inContainerReleasePath, inContainerReleasePath),
		},
	}
}

func templateRenderingContainer(instanceGroupName string, secretName string) corev1.Container {
	return corev1.Container{
		Name:  "template-render",
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
				MountPath: fmt.Sprintf("/var/run/secrets/resolved-properties/%s", instanceGroupName),
				ReadOnly:  true,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  EnvInstanceGroupName,
				Value: instanceGroupName,
			},
			{
				Name:  EnvBOSHManifestPath,
				Value: fmt.Sprintf("/var/run/secrets/resolved-properties/%s/properties.yaml", instanceGroupName),
			},
			{
				Name:  EnvJobsDir,
				Value: VolumeRenderingDataMountPath,
			},
			{
				Name: PodIPEnvVar,
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
		},
		Command: []string{
			"/bin/sh",
		},
		Args: []string{
			"-xc",
			"cf-operator util template-render",
		},
	}
}

func createDirContainer(jobs []Job) corev1.Container {
	dirs := []string{}
	for _, job := range jobs {
		jobDirs := append(job.dataDirs(job.Name), job.sysDirs(job.Name)...)
		dirs = append(dirs, jobDirs...)
	}

	return corev1.Container{
		Name:  "create-dirs",
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
		Env: []corev1.EnvVar{},
		Command: []string{
			"/bin/sh",
		},
		Args: []string{
			"-xc",
			fmt.Sprintf("mkdir -p %s", strings.Join(dirs, " ")),
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: &vcapUserID,
		},
	}
}

func boshPreStartInitContainer(
	jobName string,
	jobImage string,
	volumeMounts []corev1.VolumeMount,
	debug bool,
) corev1.Container {
	boshPreStart := filepath.Join(VolumeJobsDirMountPath, jobName, "bin", "pre-start")

	var script string
	if debug {
		script = fmt.Sprintf(`if [ -x "%[1]s" ]; then "%[1]s" || ( echo "Debug window 1hr" ; sleep 3600 ); fi`, boshPreStart)
	} else {
		script = fmt.Sprintf(`if [ -x "%[1]s" ]; then "%[1]s"; fi`, boshPreStart)
	}

	return corev1.Container{
		Name:         names.Sanitize(fmt.Sprintf("bosh-pre-start-%s", jobName)),
		Image:        jobImage,
		VolumeMounts: deduplicateVolumeMounts(volumeMounts),
		Command: []string{
			"/bin/sh",
		},
		Args: []string{
			"-xc",
			script,
		},
	}
}

func bpmPreStartInitContainer(
	process bpm.Process,
	jobImage string,
	volumeMounts []corev1.VolumeMount,
	debug bool,
) corev1.Container {

	var script string
	if debug {
		script = fmt.Sprintf(`%s || ( echo "Debug window 1hr" ; sleep 3600)`, process.Hooks.PreStart)
	} else {
		script = process.Hooks.PreStart
	}

	return corev1.Container{
		Name:         names.Sanitize(fmt.Sprintf("bpm-pre-start-%s", process.Name)),
		Image:        jobImage,
		VolumeMounts: deduplicateVolumeMounts(volumeMounts),
		Command: []string{
			"/bin/sh",
		},
		Args: []string{
			"-xc",
			script,
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: &process.Unsafe.Privileged,
		},
	}
}

func bpmProcessContainer(
	jobName string,
	processName string,
	jobImage string,
	process bpm.Process,
	volumeMounts []corev1.VolumeMount,
	healthchecks map[string]bc.HealthCheck,
) corev1.Container {
	name := names.Sanitize(fmt.Sprintf("%s-%s", jobName, processName))
	container := corev1.Container{
		Name:         names.Sanitize(name),
		Image:        jobImage,
		VolumeMounts: deduplicateVolumeMounts(volumeMounts),
		Command:      []string{process.Executable},
		Args:         process.Args,
		WorkingDir:   process.Workdir,
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: capability(process.Capabilities),
			},
			Privileged: &process.Unsafe.Privileged,
		},
		Lifecycle: &corev1.Lifecycle{},
	}

	// Setup the job drain script handler.
	drainGlob := filepath.Join(VolumeJobsDirMountPath, jobName, "bin", "drain", "*")
	container.Lifecycle.PreStop = &corev1.Handler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"sh",
				"-c",
				`for s in $(ls ` + drainGlob + `); do
					(
						echo "Running drain script $s"
						while true; do
							out=$($s)
							status=$?

							if [ "$status" -ne "0" ]; then
								echo "$s FAILED with exit code $status"
								exit $status
							fi

							if [ "$out" -lt "0" ]; then
								echo "Sleeping dynamic draining wait time for $s..."
								sleep ${out:1}
								echo "Running $s again"
							else
								echo "Sleeping static draining wait time for $s..."
								sleep $out
								echo "$s done"
								exit 0
							fi
						done
					)&
				done
				echo "Waiting..."
				wait
				echo "Done"`,
			},
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
