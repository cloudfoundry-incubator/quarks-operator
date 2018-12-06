package environment

import (
	fisv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeploymentcontroller/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Catalog provides several instances for tests
type Catalog struct{}

// DefaultBOSHManifest for tests
func (c *Catalog) DefaultBOSHManifest(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: name},
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
		ObjectMeta: v1.ObjectMeta{Name: name},
		StringData: map[string]string{},
	}
}

// InterpolateOpsConfigMap for ops interpolate configmap tests
func (c *Catalog) InterpolateOpsConfigMap(name string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{Name: name},
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
		ObjectMeta: v1.ObjectMeta{Name: name},
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
		ObjectMeta: v1.ObjectMeta{Name: name},
		StringData: map[string]string{
			"ops": `- type: remove
  path: /instance-groups/name=api
`,
		},
	}
}

// DefaultFissileCR fissile deployment CR
func (c *Catalog) DefaultFissileCR(name, manifestRef string) fisv1.BOSHDeployment {
	return fisv1.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: fisv1.BOSHDeploymentSpec{
			Manifest: fisv1.Manifest{Ref: manifestRef, Type: fisv1.ConfigMapType},
		},
	}
}

// EmptyFissileCR empty fissile deployment CR
func (c *Catalog) EmptyFissileCR(name, manifestRef string) fisv1.BOSHDeployment {
	return fisv1.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec:       fisv1.BOSHDeploymentSpec{},
	}
}

// DefaultFissileCRWithOps fissile deployment CR with ops
func (c *Catalog) DefaultFissileCRWithOps(name, manifestRef string, opsRef string) fisv1.BOSHDeployment {
	return fisv1.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: fisv1.BOSHDeploymentSpec{
			Manifest: fisv1.Manifest{Ref: manifestRef, Type: fisv1.ConfigMapType},
			Ops: []fisv1.Ops{
				{Ref: opsRef, Type: fisv1.ConfigMapType},
			},
		},
	}
}

// WrongTypeFissileCR fissile deployment CR containing wrong type
func (c *Catalog) WrongTypeFissileCR(name, manifestRef string) fisv1.BOSHDeployment {
	return fisv1.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: fisv1.BOSHDeploymentSpec{
			Manifest: fisv1.Manifest{Ref: manifestRef, Type: "wrong-type"},
		},
	}
}

// FissileCRWithWrongTypeOps fissile deployment CR with wrong type ops
func (c *Catalog) FissileCRWithWrongTypeOps(name, manifestRef string, opsRef string) fisv1.BOSHDeployment {
	return fisv1.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: fisv1.BOSHDeploymentSpec{
			Manifest: fisv1.Manifest{Ref: manifestRef, Type: fisv1.ConfigMapType},
			Ops: []fisv1.Ops{
				{Ref: opsRef, Type: "wrong-type"},
			},
		},
	}
}

// InterpolateFissileCR fissile deployment CR
func (c *Catalog) InterpolateFissileCR(name, manifestRef, opsRef string) fisv1.BOSHDeployment {
	return fisv1.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: fisv1.BOSHDeploymentSpec{
			Manifest: fisv1.Manifest{Ref: manifestRef, Type: fisv1.ConfigMapType},
			Ops: []fisv1.Ops{
				{Ref: opsRef, Type: fisv1.ConfigMapType},
				{Ref: opsRef, Type: fisv1.SecretType},
			},
		},
	}
}
