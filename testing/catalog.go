package testing

import (
	yaml "gopkg.in/yaml.v2"
	"k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	helper "code.cloudfoundry.org/cf-operator/pkg/testhelper"
	"k8s.io/apimachinery/pkg/labels"
)

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultBOSHManifest for tests
func (c *Catalog) DefaultBOSHManifest() manifest.Manifest {
	m := manifest.Manifest{}
	source := `name: foo-deployment
variables:
- name: "adminpass"
  type: "password"
  options: {is_ca: true, common_name: "some-ca"}`
	yaml.Unmarshal([]byte(source), &m)
	return m
}

// DefaultBOSHManifestConfigMap for tests
func (c *Catalog) DefaultBOSHManifestConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"manifest": `instance-groups:
- name: diego
  instances: 3
- name: mysql
`,
		},
	}
}

// DefaultSecret for tests
func (c *Catalog) DefaultSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			name: "default-value",
		},
	}
}

// DefaultConfigMap for tests
func (c *Catalog) DefaultConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			name: "default-value",
		},
	}
}

// DefaultBOSHDeployment fissile deployment CR
func (c *Catalog) DefaultBOSHDeployment(name, manifestRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
		},
	}
}

// InterpolateOpsConfigMap for ops interpolate configmap tests
func (c *Catalog) InterpolateOpsConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string]string{
			"ops": `- type: replace
  path: /instance-groups/name=diego?/instances
  value: 4
`,
		},
	}
}

// InterpolateOpsSecret for ops interpolate secret tests
func (c *Catalog) InterpolateOpsSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"ops": `- type: remove
  path: /instance-groups/name=mysql?
`,
		},
	}
}

// InterpolateOpsIncorrectSecret for ops interpolate incorrect secret tests
func (c *Catalog) InterpolateOpsIncorrectSecret(name string) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"ops": `- type: remove
  path: /instance-groups/name=api
`,
		},
	}
}

// DefaultExtendedSecret for use in tests
func (c *Catalog) DefaultExtendedSecret(name string) esv1.ExtendedSecret {
	return esv1.ExtendedSecret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esv1.ExtendedSecretSpec{
			Type:       "password",
			SecretName: "generated-secret",
		},
	}
}

// DefaultCA for use in tests
func (c *Catalog) DefaultCA(name string, ca credsgen.Certificate) corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Data: map[string][]byte{
			"ca":     ca.Certificate,
			"ca_key": ca.PrivateKey,
		},
	}
}

// DefaultExtendedStatefulSet for use in tests
func (c *Catalog) DefaultExtendedStatefulSet(name string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.DefaultStatefulSet(name),
		},
	}
}

// WrongExtendedStatefulSet for use in tests
func (c *Catalog) WrongExtendedStatefulSet(name string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			Template: c.WrongStatefulSet(name),
		},
	}
}

// OwnedReferencesExtendedStatefulSet for use in tests
func (c *Catalog) OwnedReferencesExtendedStatefulSet(name string) essv1.ExtendedStatefulSet {
	return essv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: essv1.ExtendedStatefulSetSpec{
			UpdateOnEnvChange: true,
			Template:          c.OwnedReferencesStatefulSet(name),
		},
	}
}

// DefaultStatefulSet for use in tests
func (c *Catalog) DefaultStatefulSet(name string) v1beta2.StatefulSet {
	replicaCount := int32(1)
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: &replicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"testpod": "yes",
				},
			},
			ServiceName: name,
			Template:    c.DefaultPodTemplate(name),
		},
	}
}

// WrongStatefulSet for use in tests
func (c *Catalog) WrongStatefulSet(name string) v1beta2.StatefulSet {
	replicaCount := int32(1)
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: &replicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"wrongpod": "yes",
				},
			},
			ServiceName: name,
			Template:    c.WrongPodTemplate(name),
		},
	}
}

// OwnedReferencesStatefulSet for use in tests
func (c *Catalog) OwnedReferencesStatefulSet(name string) v1beta2.StatefulSet {
	replicaCount := int32(1)
	return v1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta2.StatefulSetSpec{
			Replicas: &replicaCount,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"referencedpod": "yes",
				},
			},
			ServiceName: name,
			Template:    c.OwnedReferencesPodTemplate(name),
		},
	}
}

// DefaultPodTemplate defines a pod template with a simple web server useful for testing
func (c *Catalog) DefaultPodTemplate(name string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"testpod": "yes",
			},
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// WrongPodTemplate defines a pod template with a simple web server useful for testing
func (c *Catalog) WrongPodTemplate(name string) corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"wrongpod": "yes",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &one,
			Containers: []corev1.Container{
				{
					Name:  "wrong-container",
					Image: "wrong-image",
				},
			},
		},
	}
}

// OwnedReferencesPodTemplate defines a pod template with four references from VolumeSources, EnvFrom and Env
func (c *Catalog) OwnedReferencesPodTemplate(name string) corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"referencedpod": "yes",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &one,
			Volumes: []corev1.Volume{
				{
					Name: "secret1",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "example1",
						},
					},
				},
				{
					Name: "configmap1",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "example1",
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:    "container1",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
					EnvFrom: []corev1.EnvFromSource{
						{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "example1",
								},
							},
						},
						{
							SecretRef: &corev1.SecretEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "example1",
								},
							},
						},
					},
				},
				{
					Name:    "container2",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
					Env: []corev1.EnvVar{
						{
							Name: "ENV1",
							ValueFrom: &corev1.EnvVarSource{
								ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
									Key: "example2",
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "example2",
									},
								},
							},
						},
						{
							Name: "ENV2",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									Key: "example2",
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "example2",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// CmdPodTemplate returns the spec with a given command for busybox
func (c *Catalog) CmdPodTemplate(cmd []string) corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &one,
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: cmd,
				},
			},
		},
	}
}

// MultiContainerPodTemplate returns the spec with two containers running a given command for busybox
func (c *Catalog) MultiContainerPodTemplate(cmd []string) corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &one,
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

// FailingMultiContainerPodTemplate returns a spec with a given command for busybox and a second container which fails
func (c *Catalog) FailingMultiContainerPodTemplate(cmd []string) corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: &one,
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

// DefaultPod defines a pod with a simple web server useful for testing
func (c *Catalog) DefaultPod(name string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// LabeledPod defines a pod with labels and a simple web server
func (c *Catalog) LabeledPod(name string, labels map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// AnnotatedPod defines a pod with annotations
func (c *Catalog) AnnotatedPod(name string, annotations map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Spec: c.Sleep1hPodSpec(),
	}
}

// Sleep1hPodSpec defines a simple pod that sleeps 60*60s for testing
func (c *Catalog) Sleep1hPodSpec() corev1.PodSpec {
	one := int64(1)
	return corev1.PodSpec{
		TerminationGracePeriodSeconds: &one,
		Containers: []corev1.Container{
			{
				Name:    "busybox",
				Image:   "busybox",
				Command: []string{"sleep", "3600"},
			},
		},
	}
}

// EmptyBOSHDeployment empty fissile deployment CR
func (c *Catalog) EmptyBOSHDeployment(name, manifestRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       bdcv1.BOSHDeploymentSpec{},
	}
}

// DefaultBOSHDeploymentWithOps fissile deployment CR with ops
func (c *Catalog) DefaultBOSHDeploymentWithOps(name, manifestRef string, opsRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
			Ops: []bdcv1.Ops{
				{Ref: opsRef, Type: bdcv1.ConfigMapType},
			},
		},
	}
}

// WrongTypeBOSHDeployment fissile deployment CR containing wrong type
func (c *Catalog) WrongTypeBOSHDeployment(name, manifestRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: "wrong-type"},
		},
	}
}

// BOSHDeploymentWithWrongTypeOps fissile deployment CR with wrong type ops
func (c *Catalog) BOSHDeploymentWithWrongTypeOps(name, manifestRef string, opsRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
			Ops: []bdcv1.Ops{
				{Ref: opsRef, Type: "wrong-type"},
			},
		},
	}
}

// InterpolateBOSHDeployment fissile deployment CR
func (c *Catalog) InterpolateBOSHDeployment(name, manifestRef, opsRef string, secretRef string) bdcv1.BOSHDeployment {
	return bdcv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdcv1.BOSHDeploymentSpec{
			Manifest: bdcv1.Manifest{Ref: manifestRef, Type: bdcv1.ConfigMapType},
			Ops: []bdcv1.Ops{
				{Ref: opsRef, Type: bdcv1.ConfigMapType},
				{Ref: secretRef, Type: bdcv1.SecretType},
			},
		},
	}
}

// DefaultExtendedJob default values
func (c *Catalog) DefaultExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "1"},
	)
}

// LongRunningExtendedJob has a longer sleep time
func (c *Catalog) LongRunningExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "15"},
	)
}

// OnDeleteExtendedJob runs for deleted pods
func (c *Catalog) OnDeleteExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateDeleted,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{},
		[]string{"sleep", "1"},
	)
}

// MatchExpressionExtendedJob uses Matchexpressions for matching
func (c *Catalog) MatchExpressionExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{},
		[]*ejv1.Requirement{
			{Key: "env", Operator: "in", Values: []string{"production"}},
		},
		[]string{"sleep", "1"},
	)
}

// ComplexMatchExtendedJob uses MatchLabels and MatchExpressions
func (c *Catalog) ComplexMatchExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]*ejv1.Requirement{
			{Key: "env", Operator: "in", Values: []string{"production"}},
		},
		[]string{"sleep", "1"},
	)
}

// OutputExtendedJob persists its output
func (c *Catalog) OutputExtendedJob(name string, template corev1.PodTemplateSpec) *ejv1.ExtendedJob {
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: &ejv1.Trigger{
				Strategy: "podstate",
				PodState: &ejv1.PodStateTrigger{
					When:     "ready",
					Selector: &ejv1.Selector{MatchLabels: &labels.Set{"key": "value"}},
				},
			},
			Template: template,
			Output: &ejv1.Output{
				NamePrefix:   name + "-output-",
				OutputType:   "json",
				SecretLabels: map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			},
		},
	}
}

// LabelTriggeredExtendedJob allows customization of labels triggers
func (c *Catalog) LabelTriggeredExtendedJob(name string, state ejv1.PodState, ml labels.Set, me []*ejv1.Requirement, cmd []string) *ejv1.ExtendedJob {
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Trigger: &ejv1.Trigger{
				Strategy: "podstate",
				PodState: &ejv1.PodStateTrigger{
					When: state,
					Selector: &ejv1.Selector{
						MatchLabels:      &ml,
						MatchExpressions: me,
					},
				},
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// DefaultExtendedJobWithSucceededJob returns an ExtendedJob and a Job owned by it
func (c *Catalog) DefaultExtendedJobWithSucceededJob(name string) (*ejv1.ExtendedJob, *batchv1.Job, *corev1.Pod) {
	ejob := c.DefaultExtendedJob(name)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-job",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       name,
					UID:        "",
					Controller: helper.Bool(true),
				},
			},
		},
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
			Trigger: &ejv1.Trigger{
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
			Trigger: &ejv1.Trigger{
				Strategy: ejv1.TriggerOnce,
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}
