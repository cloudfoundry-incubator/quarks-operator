package bpmconverter

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"

	"code.cloudfoundry.org/cf-operator/container-run/pkg/containerrun"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/bpm"
	"code.cloudfoundry.org/cf-operator/pkg/bosh/disk"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/operatorimage"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	log "code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

var (
	rootUserID = int64(0)
	vcapUserID = int64(1000)
	entrypoint = []string{"/usr/bin/dumb-init", "--"}
)

const (
	// EnvJobsDir is a key for the container Env used to lookup the jobs dir.
	EnvJobsDir = "JOBS_DIR"

	// EnvLogsDir is the path from where to tail file logs.
	EnvLogsDir = "LOGS_DIR"
)

// ContainerFactoryImpl is a concrete implementation of ContainerFactor.
type ContainerFactoryImpl struct {
	deploymentName       string
	instanceGroupName    string
	version              string
	disableLogSidecar    bool
	releaseImageProvider bdm.ReleaseImageProvider
	bpmConfigs           bpm.Configs
}

// NewContainerFactory returns a concrete implementation of ContainerFactory.
func NewContainerFactory(deploymentName string, instanceGroupName string, version string, disableLogSidecar bool, releaseImageProvider bdm.ReleaseImageProvider, bpmConfigs bpm.Configs) *ContainerFactoryImpl {
	return &ContainerFactoryImpl{
		deploymentName:       deploymentName,
		instanceGroupName:    instanceGroupName,
		version:              version,
		disableLogSidecar:    disableLogSidecar,
		releaseImageProvider: releaseImageProvider,
		bpmConfigs:           bpmConfigs,
	}
}

// JobsToInitContainers creates a list of Containers for corev1.PodSpec InitContainers field.
func (c *ContainerFactoryImpl) JobsToInitContainers(
	jobs []bdm.Job,
	defaultVolumeMounts []corev1.VolumeMount,
	bpmDisks disk.BPMResourceDisks,
	requiredService *string,
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
			copyingSpecsInitContainer := JobSpecCopierContainer(job.Release, jobImage, VolumeRenderingDataName)
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
					job.Properties.Quarks.Debug,
					job.Properties.Quarks.Run.SecurityContext.DeepCopy(),
				)

				bpmPreStartInitContainers = append(bpmPreStartInitContainers, *container.DeepCopy())
			}
		}

		// Setup the BOSH pre-start init container for the job.
		boshPreStartInitContainer := boshPreStartInitContainer(
			job.Name,
			jobImage,
			append(defaultVolumeMounts, bpmDisks.VolumeMounts()...),
			job.Properties.Quarks.Debug,
			job.Properties.Quarks.Run.SecurityContext.DeepCopy(),
		)
		boshPreStartInitContainers = append(boshPreStartInitContainers, *boshPreStartInitContainer.DeepCopy())
	}

	resolvedPropertiesSecretName := names.InstanceGroupSecretName(
		names.DeploymentSecretTypeInstanceGroupResolvedProperties, // ig-resolved
		c.deploymentName,
		c.instanceGroupName,
		c.version,
	)

	initContainers := flattenContainers(
		containerRunCopier(),
		copyingSpecsInitContainers,
		templateRenderingContainer(c.deploymentName, c.instanceGroupName, resolvedPropertiesSecretName),
		createDirContainer(jobs, c.instanceGroupName, c.deploymentName),
		createWaitContainer(requiredService),
		boshPreStartInitContainers,
		bpmPreStartInitContainers,
	)

	return initContainers, nil
}

func createWaitContainer(requiredService *string) []corev1.Container {
	if requiredService == nil {
		return nil
	}
	return []corev1.Container{{
		Name:    "wait-for",
		Image:   operatorimage.GetOperatorDockerImage(),
		Command: []string{"/usr/bin/dumb-init", "--"},
		Args: []string{
			"/bin/sh",
			"-xc",
			fmt.Sprintf("cf-operator util wait %s", *requiredService),
		},
	}}

}

// JobsToContainers creates a list of Containers for corev1.PodSpec Containers field.
func (c *ContainerFactoryImpl) JobsToContainers(
	jobs []bdm.Job,
	defaultVolumeMounts []corev1.VolumeMount,
	bpmDisks disk.BPMResourceDisks,
) ([]corev1.Container, error) {
	var containers []corev1.Container

	if len(jobs) == 0 {
		return nil, errors.Errorf("instance group '%s' has no jobs defined", c.instanceGroupName)
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

		for processIndex, process := range bpmConfig.Processes {
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

			// The post-start script should be executed only once per job, so we set it up in the first
			// process container.
			var postStart postStart
			if processIndex == 0 {
				conditionProperty := job.Properties.Quarks.PostStart.Condition
				if conditionProperty != nil && conditionProperty.Exec != nil && len(conditionProperty.Exec.Command) > 0 {
					postStart.condition = &containerrun.Command{
						Name: conditionProperty.Exec.Command[0],
						Arg:  conditionProperty.Exec.Command[1:],
					}
				}

				postStart.command = &containerrun.Command{
					Name: filepath.Join(VolumeJobsDirMountPath, job.Name, "bin", "post-start"),
				}
			}

			container := bpmProcessContainer(
				job.Name,
				process.Name,
				jobImage,
				process,
				processVolumeMounts,
				job.Properties.Quarks.Run.HealthCheck,
				job.Properties.Quarks.Envs,
				job.Properties.Quarks.Run.SecurityContext.DeepCopy(),
				postStart,
			)

			containers = append(containers, *container.DeepCopy())
		}
	}

	// When disableLogSidecar is true, it will stop
	// appending the sidecar, default behaviour is to
	// colocate it always in the pod.
	if !c.disableLogSidecar {
		logsTailer := logsTailerContainer(c.instanceGroupName)
		containers = append(containers, logsTailer)
	}

	return containers, nil
}

// logsTailerContainer is a container that tails all logs in /var/vcap/sys/log.
func logsTailerContainer(instanceGroupName string) corev1.Container {
	return corev1.Container{
		Name:            "logs",
		Image:           operatorimage.GetOperatorDockerImage(),
		ImagePullPolicy: operatorimage.GetOperatorImagePullPolicy(),
		VolumeMounts:    []corev1.VolumeMount{*sysDirVolumeMount()},
		Args: []string{
			"util",
			"tail-logs",
		},
		Env: []corev1.EnvVar{
			{
				Name:  EnvLogsDir,
				Value: "/var/vcap/sys/log",
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: &rootUserID,
		},
	}
}

func containerRunCopier() corev1.Container {
	dstDir := fmt.Sprintf("%s/container-run", VolumeRenderingDataMountPath)
	return corev1.Container{
		Name:            "container-run-copier",
		Image:           operatorimage.GetOperatorDockerImage(),
		ImagePullPolicy: operatorimage.GetOperatorImagePullPolicy(),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      VolumeRenderingDataName,
				MountPath: VolumeRenderingDataMountPath,
			},
		},
		Command: entrypoint,
		Args: []string{
			"/bin/sh",
			"-c",
			fmt.Sprintf(`
				set -o errexit
				mkdir -p '%[1]s'
				cp /usr/local/bin/container-run '%[1]s'/container-run
			`, dstDir),
		},
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
			fmt.Sprintf("mkdir -p %s && cp -ar %s/* %s && chown vcap:vcap %s -R", inContainerReleasePath, VolumeJobsSrcDirMountPath, inContainerReleasePath, inContainerReleasePath),
		},
	}
}

func templateRenderingContainer(deploymentName string, instanceGroupName string, secretName string) corev1.Container {
	return corev1.Container{
		Name:            "template-render",
		Image:           operatorimage.GetOperatorDockerImage(),
		ImagePullPolicy: operatorimage.GetOperatorImagePullPolicy(),
		VolumeMounts: []corev1.VolumeMount{
			*renderingVolumeMount(),
			*jobsDirVolumeMount(),
			resolvedPropertiesVolumeMount(secretName, instanceGroupName),
		},
		Env: []corev1.EnvVar{
			{
				Name:  EnvDeploymentName,
				Value: deploymentName,
			},
			{
				Name:  EnvInstanceGroupName,
				Value: instanceGroupName,
			},
			{
				Name:  qjv1a1.RemoteIDKey,
				Value: instanceGroupName,
			},
			{
				Name:  EnvBOSHManifestPath,
				Value: fmt.Sprintf(resolvedPropertiesFormat+"/properties.yaml", instanceGroupName),
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
		Command: entrypoint,
		Args: []string{
			"/bin/sh",
			"-xc",
			"cf-operator util template-render",
		},
	}
}

func createDirContainer(jobs []bdm.Job, instanceGroupName, manifestName string) corev1.Container {
	dirs := []string{}
	for _, job := range jobs {
		jobDirs := append(job.DataDirs(), job.SysDirs()...)
		dirs = append(dirs, jobDirs...)
	}

	return corev1.Container{
		Name:            "create-dirs",
		Image:           operatorimage.GetOperatorDockerImage(),
		ImagePullPolicy: operatorimage.GetOperatorImagePullPolicy(),
		VolumeMounts: []corev1.VolumeMount{
			corev1.VolumeMount{
				Name: VolumeDataDirName(
					manifestName,
					instanceGroupName),
				MountPath: VolumeDataDirMountPath,
			},
			*sysDirVolumeMount(),
		},
		Command: entrypoint,
		Args: []string{
			"/bin/sh",
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
	securityContext *corev1.SecurityContext,
) corev1.Container {
	boshPreStart := filepath.Join(VolumeJobsDirMountPath, jobName, "bin", "pre-start")

	var script string
	if debug {
		script = fmt.Sprintf(`if [ -x "%[1]s" ]; then "%[1]s" || ( echo "Debug window 1hr" ; sleep 3600 ); fi`, boshPreStart)
	} else {
		script = fmt.Sprintf(`if [ -x "%[1]s" ]; then "%[1]s"; fi`, boshPreStart)
	}

	if securityContext == nil {
		securityContext = &corev1.SecurityContext{}
	}
	securityContext.RunAsUser = &rootUserID

	return corev1.Container{
		Name:         names.Sanitize(fmt.Sprintf("bosh-pre-start-%s", jobName)),
		Image:        jobImage,
		VolumeMounts: deduplicateVolumeMounts(volumeMounts),
		Command:      entrypoint,
		Args: []string{
			"/bin/sh",
			"-xc",
			script,
		},
		SecurityContext: securityContext,
	}
}

func bpmPreStartInitContainer(
	process bpm.Process,
	jobImage string,
	volumeMounts []corev1.VolumeMount,
	debug bool,
	securityContext *corev1.SecurityContext,
) corev1.Container {
	var script string
	if debug {
		script = fmt.Sprintf(`%s || ( echo "Debug window 1hr" ; sleep 3600 )`, process.Hooks.PreStart)
	} else {
		script = process.Hooks.PreStart
	}

	if securityContext == nil {
		securityContext = &corev1.SecurityContext{}
	}
	if securityContext.Capabilities == nil && len(process.Capabilities) > 0 {
		securityContext.Capabilities = &corev1.Capabilities{
			Add: capability(process.Capabilities),
		}
	}
	if securityContext.Privileged == nil {
		securityContext.Privileged = &process.Unsafe.Privileged
	}
	securityContext.RunAsUser = &rootUserID

	return corev1.Container{
		Name:         names.Sanitize(fmt.Sprintf("bpm-pre-start-%s", process.Name)),
		Image:        jobImage,
		VolumeMounts: deduplicateVolumeMounts(volumeMounts),
		Command:      entrypoint,
		Args: []string{
			"/bin/sh",
			"-xc",
			script,
		},
		SecurityContext: securityContext,
	}
}

type postStart struct {
	command, condition *containerrun.Command
}

func bpmProcessContainer(
	jobName string,
	processName string,
	jobImage string,
	process bpm.Process,
	volumeMounts []corev1.VolumeMount,
	healthchecks map[string]bdm.HealthCheck,
	quarksEnvs []corev1.EnvVar,
	securityContext *corev1.SecurityContext,
	postStart postStart,
) corev1.Container {
	name := names.Sanitize(fmt.Sprintf("%s-%s", jobName, processName))

	if securityContext == nil {
		securityContext = &corev1.SecurityContext{}
	}
	if securityContext.Capabilities == nil && len(process.Capabilities) > 0 {
		securityContext.Capabilities = &corev1.Capabilities{
			Add: capability(process.Capabilities),
		}
	}
	if securityContext.Privileged == nil {
		securityContext.Privileged = &process.Unsafe.Privileged
	}
	if securityContext.RunAsUser == nil {
		securityContext.RunAsUser = &rootUserID
	}

	workdir := process.Workdir
	if workdir == "" {
		workdir = filepath.Join(VolumeJobsDirMountPath, jobName)
	}
	command, args := generateBPMCommand(&process, postStart)
	limits := corev1.ResourceList{}
	if process.Limits.Memory != "" {
		quantity, err := resource.ParseQuantity(process.Limits.Memory)
		if err != nil {
			log.Errorf(context.TODO(), "Error parsing %s: %v", process.Limits.Memory, err)
		} else {
			limits[corev1.ResourceMemory] = quantity
		}
	}
	container := corev1.Container{
		Name:            names.Sanitize(name),
		Image:           jobImage,
		VolumeMounts:    deduplicateVolumeMounts(volumeMounts),
		Command:         command,
		Args:            args,
		Env:             process.NewEnvs(quarksEnvs),
		WorkingDir:      workdir,
		SecurityContext: securityContext,
		Lifecycle:       &corev1.Lifecycle{},
		Resources: corev1.ResourceRequirements{
			Requests: process.Requests,
			Limits:   limits,
		},
	}

	// Setup the job drain script handler.
	drainGlob := filepath.Join(VolumeJobsDirMountPath, jobName, "bin", "drain", "*")
	container.Lifecycle.PreStop = &corev1.Handler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"/bin/sh",
				"-c",
				`
shopt -s nullglob
for s in ` + drainGlob + `; do
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
echo "Waiting for subprocesses to finish..."
wait
echo "Done"`,
			},
		},
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

// capability converts string slice into Capability slice of kubernetes.
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

// generateArgs generates the bpm container arguments.
func generateBPMCommand(
	process *bpm.Process,
	postStart postStart,
) ([]string, []string) {
	command := []string{"/usr/bin/dumb-init", "--"}
	args := []string{fmt.Sprintf("%s/container-run/container-run", VolumeRenderingDataMountPath)}
	if postStart.command != nil {
		args = append(args, "--post-start-name", postStart.command.Name)
		if postStart.condition != nil {
			args = append(args, "--post-start-condition-name", postStart.condition.Name)
			for _, arg := range postStart.condition.Arg {
				args = append(args, "--post-start-condition-arg", arg)
			}
		}
	}
	args = append(args, "--")
	args = append(args, process.Executable)
	args = append(args, process.Args...)

	return command, args
}
