package testing

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
)

// CmdPodTemplate returns the spec with a given command for busybox
func (c *Catalog) CmdPodTemplate(cmd []string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: cmd,
					Env: []corev1.EnvVar{
						{Name: "REPLICAS", Value: "1"},
						{Name: "AZ_INDEX", Value: "1"},
						{Name: "POD_ORDINAL", Value: "0"},
					},
				},
			},
		},
	}
}

// ConfigPodTemplate returns the spec with a given command for busybox
func (c *Catalog) ConfigPodTemplate() corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"delete": "pod"},
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &one,
			Volumes: []corev1.Volume{
				{
					Name: "secret1",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "secret1",
						},
					},
				},
				{
					Name: "configmap1",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "config1",
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "1"},
					Env: []corev1.EnvVar{
						{Name: "REPLICAS", Value: "1"},
						{Name: "AZ_INDEX", Value: "1"},
						{Name: "POD_ORDINAL", Value: "0"},
					},
				},
			},
		},
	}
}

// FailingMultiContainerPodTemplate returns a spec with a given command for busybox and a second container which fails
func (c *Catalog) FailingMultiContainerPodTemplate(cmd []string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: cmd,
				},
				{
					Name:    "failing",
					Image:   "busybox",
					Command: []string{"exit", "1"},
				},
			},
		},
	}
}

// MultiContainerPodTemplate returns the spec with two containers running a given command for busybox
func (c *Catalog) MultiContainerPodTemplate(cmd []string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: util.Int64(1),
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: cmd,
				},
				{
					Name:    "busybox2",
					Image:   "busybox",
					Command: cmd,
				},
			},
		},
	}
}

// DefaultExtendedJob default values
func (c *Catalog) DefaultExtendedJob(name string) *ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerNow,
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// DefaultExtendedJobWithSucceededJob returns an ExtendedJob and a Job owned by it
func (c *Catalog) DefaultExtendedJobWithSucceededJob(name string) (*ejv1.ExtendedJob, *batchv1.Job, *corev1.Pod) {
	ejob := c.DefaultExtendedJob(name)
	backoffLimit := util.Int32(2)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-job",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       name,
					UID:        "",
					Controller: util.Bool(true),
				},
			},
		},
		Spec:   batchv1.JobSpec{BackoffLimit: backoffLimit},
		Status: batchv1.JobStatus{Succeeded: 1},
	}
	pod := c.DefaultPod(name + "-pod")
	pod.Labels = map[string]string{
		"job-name": job.GetName(),
	}
	return ejob, job, &pod
}

// ErrandExtendedJob default values
func (c *Catalog) ErrandExtendedJob(name string) ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerNow,
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// AutoErrandExtendedJob default values
func (c *Catalog) AutoErrandExtendedJob(name string) ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}
