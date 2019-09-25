package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
)

// DefaultExtendedStatefulSet for use in tests
func (c *Catalog) DefaultExtendedStatefulSet(name string) esv1.ExtendedStatefulSet {
	return esv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esv1.ExtendedStatefulSetSpec{
			Template: c.DefaultStatefulSet(name),
		},
	}
}

// WrongExtendedStatefulSet for use in tests
func (c *Catalog) WrongExtendedStatefulSet(name string) esv1.ExtendedStatefulSet {
	return esv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esv1.ExtendedStatefulSetSpec{
			Template: c.WrongStatefulSet(name),
		},
	}
}

// ExtendedStatefulSetWithPVC for use in tests
func (c *Catalog) ExtendedStatefulSetWithPVC(name, pvcName string, storageClassName string) esv1.ExtendedStatefulSet {
	return esv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esv1.ExtendedStatefulSetSpec{
			Template: c.StatefulSetWithPVC(name, pvcName, storageClassName),
		},
	}
}

// WrongExtendedStatefulSetWithPVC for use in tests
func (c *Catalog) WrongExtendedStatefulSetWithPVC(name, pvcName string, storageClassName string) esv1.ExtendedStatefulSet {
	return esv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esv1.ExtendedStatefulSetSpec{
			Template: c.WrongStatefulSetWithPVC(name, pvcName, storageClassName),
		},
	}
}

// OwnedReferencesExtendedStatefulSet for use in tests
func (c *Catalog) OwnedReferencesExtendedStatefulSet(name string) esv1.ExtendedStatefulSet {
	return esv1.ExtendedStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: esv1.ExtendedStatefulSetSpec{
			UpdateOnConfigChange: true,
			Template:             c.OwnedReferencesStatefulSet(name),
		},
	}
}
