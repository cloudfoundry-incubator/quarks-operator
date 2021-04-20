package bpmconverter

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"

	"code.cloudfoundry.org/quarks-operator/pkg/bosh/bpm"
	bdm "code.cloudfoundry.org/quarks-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/logrotate"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/util/operatorimage"
	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

// JobsToContainers creates a list of Containers for corev1.PodSpec Containers field.
func (c *ContainerFactoryImpl) JobsToContainers(
	jobs []bdm.Job,
	defaultVolumeMounts []corev1.VolumeMount,
	bpmDisks bdm.Disks,
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

			// The post-start script should be executed only once per job, so we set it up in the first
			// process container.
			var postStart postStart
			if processIndex == 0 {
				conditionProperty := bpmConfig.PostStart.Condition
				if conditionProperty != nil && conditionProperty.Exec != nil && len(conditionProperty.Exec.Command) > 0 {
					postStart.condition = &postStartCmd{
						Name: conditionProperty.Exec.Command[0],
						Arg:  conditionProperty.Exec.Command[1:],
					}
				}

				postStart.command = &postStartCmd{
					Name: filepath.Join(VolumeJobsDirMountPath, job.Name, "bin", "post-start"),
				}
			}

			container, err := bpmProcessContainer(
				job.Name,
				process.Name,
				jobImage,
				process,
				proccessVolumentMounts(defaultVolumeMounts, processDisks, ephemeralMount, persistentDiskMount),
				bpmConfig.Run.HealthCheck,
				job.Properties.Quarks.Envs,
				bpmConfig.Run.SecurityContext.DeepCopy(),
				postStart,
				strconv.Itoa(len(bpmConfig.Processes)),
			)
			if err != nil {
				return []corev1.Container{}, err
			}

			containers = append(containers, *container.DeepCopy())
		}
	}

	// When disableLogSidecar is true, it will stop
	// appending the sidecar, default behaviour is to
	// colocate it always in the pod.
	if !c.disableLogSidecar {
		logsTailer := logsTailerContainer()
		containers = append(containers, logsTailer)
	}

	return containers, nil
}

// logsTailerContainer is a container that tails all logs in /var/vcap/sys/log.
func logsTailerContainer() corev1.Container {
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
			{
				Name:  "LOGROTATE_INTERVAL",
				Value: strconv.Itoa(logrotate.GetInterval()),
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser: &rootUserID,
		},
	}
}

// Command represents a command to be run.
type postStartCmd struct {
	Name string
	Arg  []string
}
type postStart struct {
	command, condition *postStartCmd
}

func bpmProcessContainer(
	jobName string,
	processName string,
	jobImage string,
	process bpm.Process,
	volumeMounts []corev1.VolumeMount,
	healthChecks map[string]bpm.HealthCheck,
	quarksEnvs []corev1.EnvVar,
	securityContext *corev1.SecurityContext,
	postStart postStart,
	processCount string,
) (corev1.Container, error) {
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
	command, args := generateBPMCommand(jobName, &process, postStart)

	limits := corev1.ResourceList{}
	if process.Limits.Memory != "" {
		quantity, err := resource.ParseQuantity(process.Limits.Memory)
		if err != nil {
			return corev1.Container{}, fmt.Errorf("error parsing memory limit '%s': %v", process.Limits.Memory, err)
		}
		limits[corev1.ResourceMemory] = quantity
	}
	if process.Limits.CPU != "" {
		quantity, err := resource.ParseQuantity(process.Limits.CPU)
		if err != nil {
			return corev1.Container{}, fmt.Errorf("error parsing cpu limit '%s': %v", process.Limits.CPU, err)
		}
		limits[corev1.ResourceCPU] = quantity
	}

	newEnvs := process.NewEnvs(quarksEnvs)
	newEnvs = defaultEnv(newEnvs, map[string]corev1.EnvVar{
		EnvPodOrdinal: podOrdinalEnv,
		EnvReplicas:   replicasEnv,
		EnvAzIndex:    azIndexEnv,
	})

	container := corev1.Container{
		Name:            names.Sanitize(name),
		Image:           jobImage,
		VolumeMounts:    deduplicateVolumeMounts(volumeMounts),
		Command:         command,
		Args:            args,
		Env:             newEnvs,
		WorkingDir:      workdir,
		SecurityContext: securityContext,
		Lifecycle:       &corev1.Lifecycle{},
		Resources: corev1.ResourceRequirements{
			Requests: process.Requests,
			Limits:   limits,
		},
	}

	// Setup the job drain script handler.
	drainScript := filepath.Join(VolumeJobsDirMountPath, jobName, "bin", "drain")
	container.Lifecycle.PreStop = &corev1.Handler{
		Exec: &corev1.ExecAction{
			Command: []string{
				"/bin/sh",
				"-c",
				`
shopt -s nullglob
waitExit() {
	e="$1"
	touch /mnt/drain-stamps/` + container.Name + `
	echo "Waiting for other drain scripts to finish."
	while [ $(ls -1 /mnt/drain-stamps | wc -l) -lt ` + processCount + ` ]; do sleep 5; done
	exit "$e"
}
s="` + drainScript + `"
if [ ! -x "$s" ]; then
	waitExit 0
fi
echo "Running drain script $s"
while true; do
	out=$( $s )
	status=$?

	if [ "$status" -ne "0" ]; then
		echo "$s FAILED with exit code $status"
		waitExit $status
	fi

	if [ "$out" -lt "0" ]; then
		echo "Sleeping dynamic draining wait time for $s..."
		sleep ${out:1}
		echo "Running $s again"
	else
		echo "Sleeping static draining wait time for $s..."
		sleep $out
		echo "$s done"
		waitExit 0
	fi
done
echo "Done"`,
			},
		},
	}

	for name, hc := range healthChecks {
		if name == process.Name {
			if hc.ReadinessProbe != nil {
				container.ReadinessProbe = hc.ReadinessProbe
			}
			if hc.LivenessProbe != nil {
				container.LivenessProbe = hc.LivenessProbe
			}
		}
	}
	return container, nil
}

// defaultEnv adds the default value if no value is set
func defaultEnv(envs []corev1.EnvVar, defaults map[string]corev1.EnvVar) []corev1.EnvVar {
	for _, env := range envs {
		delete(defaults, env.Name)
	}

	for _, env := range defaults {
		envs = append(envs, env)
	}
	return envs
}

// generateBPMCommand generates the bpm container arguments.
func generateBPMCommand(
	jobName string,
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
	args = append(args, "--job-name", jobName)
	args = append(args, "--process-name", process.Name)
	args = append(args, "--")
	args = append(args, process.Executable)
	args = append(args, process.Args...)

	return command, args
}
