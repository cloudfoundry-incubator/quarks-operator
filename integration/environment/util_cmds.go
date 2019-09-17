package environment

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
)

var (
	logsDir     = "LOGS_DIR"
	logsDirPath = "/var/vcap/sys/log"
)

// CreatePodWithTailLogsContainer will generate a pod with two containers
// One is the parent container that will execute a cmd, preferrable something
// that writes into files under /var/vcap/sys/log
// The side-car container, will be tailing the logs of specific files under
// /var/vcap/sys/log, by running the cf-operator util tail-logs subcmommand
func (m *Machine) CreatePodWithTailLogsContainer(podName string, parentPodCmd string, parentCName string, sidecardCName string,
	dockerImg string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyOnFailure,
			Containers: []corev1.Container{
				{
					Name:  parentCName,
					Image: dockerImg,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      converter.VolumeSysDirName,
							MountPath: converter.VolumeSysDirMountPath,
						},
					},
					Command: []string{
						"/bin/sh",
					},
					Args: []string{
						"-xc",
						parentPodCmd,
					},
				},
				{
					Name:  sidecardCName,
					Image: dockerImg,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      converter.VolumeSysDirName,
							MountPath: converter.VolumeSysDirMountPath,
						},
					},
					Args: []string{
						"util",
						"tail-logs",
					},
					Env: []corev1.EnvVar{
						{
							Name:  logsDir,
							Value: logsDirPath,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name:         converter.VolumeSysDirName,
					VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
				},
			},
		},
	}
}
