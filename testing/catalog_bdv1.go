package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
)

// DefaultBOSHDeployment a deployment CR
func (c *Catalog) DefaultBOSHDeployment(name, manifestRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
		},
	}
}

// SecretBOSHDeployment a deployment CR which expects the BOSH manifest in a secret.
// The name needs to match the name inside the referenced manifest.
func (c *Catalog) SecretBOSHDeployment(name, manifestRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.SecretReference},
		},
	}
}

// EmptyBOSHDeployment empty deployment CR
func (c *Catalog) EmptyBOSHDeployment(name, manifestRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       bdv1.BOSHDeploymentSpec{},
	}
}

// DefaultBOSHDeploymentWithOps a deployment CR with ops
func (c *Catalog) DefaultBOSHDeploymentWithOps(name, manifestRef string, opsRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
			Ops: []bdv1.ResourceReference{
				{Name: opsRef, Type: bdv1.ConfigMapReference},
			},
		},
	}
}

// WrongTypeBOSHDeployment a deployment CR containing wrong type
func (c *Catalog) WrongTypeBOSHDeployment(name, manifestRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: "wrong-type"},
		},
	}
}

// BOSHDeploymentWithWrongTypeOps a deployment CR with wrong type ops
func (c *Catalog) BOSHDeploymentWithWrongTypeOps(name, manifestRef string, opsRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
			Ops: []bdv1.ResourceReference{
				{Name: opsRef, Type: "wrong-type"},
			},
		},
	}
}

// InterpolateBOSHDeployment a deployment CR
func (c *Catalog) InterpolateBOSHDeployment(name, manifestRef, opsRef string, secretRef string) bdv1.BOSHDeployment {
	return bdv1.BOSHDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: bdv1.BOSHDeploymentSpec{
			Manifest: bdv1.ResourceReference{Name: manifestRef, Type: bdv1.ConfigMapReference},
			Ops: []bdv1.ResourceReference{
				{Name: opsRef, Type: bdv1.ConfigMapReference},
				{Name: secretRef, Type: bdv1.SecretReference},
			},
		},
	}
}
