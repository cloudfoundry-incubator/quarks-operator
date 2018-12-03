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

// DefaultFissileCR fissile deployment CR
func (c *Catalog) DefaultFissileCR(name, manifestRef string) fisv1.BOSHDeployment {
	return fisv1.BOSHDeployment{
		ObjectMeta: v1.ObjectMeta{Name: name},
		Spec: fisv1.BOSHDeploymentSpec{
			Manifest: fisv1.Manifest{Ref: manifestRef, Type: fisv1.ConfigMapType},
		},
	}
}
