package testing

import (
	v1beta1 "k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bdcv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
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
		StringData: map[string]string{},
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

// DefaultPodTemplate defines a pod template with a simple web server useful for testing
func (c *Catalog) DefaultPodTemplate(name string) corev1.PodTemplateSpec {
	one := int64(1)
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"testpod": "yes",
			},
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &one,
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
			},
		},
	}
}

// DefaultPod defines a pod with a simple web server useful for testing
func (c *Catalog) DefaultPod(name string) corev1.Pod {
	one := int64(1)
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &one,
			Containers: []corev1.Container{
				{
					Name:    "busybox",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
				},
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
