package testing

import (
	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/cf-operator/pkg/credsgen"
	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
)

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultBOSHManifest for tests
func (c *Catalog) DefaultBOSHManifest(name string) corev1.ConfigMap {
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
			Template: c.OwnedReferencesStatefulSet(name),
		},
	}
}

// DefaultStatefulSet for use in tests
func (c *Catalog) DefaultStatefulSet(name string) v1beta1.StatefulSet {
	replicaCount := int32(1)
	return v1beta1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.StatefulSetSpec{
			Replicas:    &replicaCount,
			ServiceName: name,
			Template:    c.DefaultPodTemplate(name),
		},
	}
}

// WrongStatefulSet for use in tests
func (c *Catalog) WrongStatefulSet(name string) v1beta1.StatefulSet {
	replicaCount := int32(1)
	return v1beta1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.StatefulSetSpec{
			Replicas:    &replicaCount,
			ServiceName: name,
			Template:    c.WrongPodTemplate(name),
		},
	}
}

// OwnedReferencesStatefulSet for use in tests
func (c *Catalog) OwnedReferencesStatefulSet(name string) v1beta1.StatefulSet {
	replicaCount := int32(1)
	return v1beta1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1beta1.StatefulSetSpec{
			Replicas:    &replicaCount,
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
		[]string{"sleep", "1"},
	)
}

// LongRunningExtendedJob has a longer sleep time
func (c *Catalog) LongRunningExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateReady,
		map[string]string{"key": "value"},
		[]string{"sleep", "15"},
	)
}

// OnDeleteExtendedJob runs for deleted pods
func (c *Catalog) OnDeleteExtendedJob(name string) *ejv1.ExtendedJob {
	return c.LabelTriggeredExtendedJob(
		name,
		ejv1.PodStateDeleted,
		map[string]string{"key": "value"},
		[]string{"sleep", "1"},
	)
}

// LabelTriggeredExtendedJob allows customization of labels triggers
func (c *Catalog) LabelTriggeredExtendedJob(name string, state ejv1.PodState, ml map[string]string, cmd []string) *ejv1.ExtendedJob {
	return &ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Triggers: ejv1.Triggers{
				When:     state,
				Selector: ejv1.Selector{MatchLabels: ml},
			},
			Template: c.CmdPodTemplate(cmd),
		},
	}
}

// ErrandExtendedJob default values
func (c *Catalog) ErrandExtendedJob(name string) ejv1.ExtendedJob {
	cmd := []string{"sleep", "1"}
	return ejv1.ExtendedJob{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: ejv1.ExtendedJobSpec{
			Run:      ejv1.RunNow,
			Template: c.CmdPodTemplate(cmd),
		},
	}
}
