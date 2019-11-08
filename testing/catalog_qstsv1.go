package testing

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
)

// DefaultQuarksStatefulSet for use in tests
func (c *Catalog) DefaultQuarksStatefulSet(name string) qstsv1a1.QuarksStatefulSet {
	return qstsv1a1.QuarksStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qstsv1a1.QuarksStatefulSetSpec{
			Template: c.DefaultStatefulSet(name),
		},
	}
}

// WrongQuarksStatefulSet for use in tests
func (c *Catalog) WrongQuarksStatefulSet(name string) qstsv1a1.QuarksStatefulSet {
	return qstsv1a1.QuarksStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qstsv1a1.QuarksStatefulSetSpec{
			Template: c.WrongStatefulSet(name),
		},
	}
}

// QuarksStatefulSetWithPVC for use in tests
func (c *Catalog) QuarksStatefulSetWithPVC(name, pvcName string, storageClassName string) qstsv1a1.QuarksStatefulSet {
	return qstsv1a1.QuarksStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qstsv1a1.QuarksStatefulSetSpec{
			Template: c.StatefulSetWithPVC(name, pvcName, storageClassName),
		},
	}
}

// WrongQuarksStatefulSetWithPVC for use in tests
func (c *Catalog) WrongQuarksStatefulSetWithPVC(name, pvcName string, storageClassName string) qstsv1a1.QuarksStatefulSet {
	return qstsv1a1.QuarksStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qstsv1a1.QuarksStatefulSetSpec{
			Template: c.WrongStatefulSetWithPVC(name, pvcName, storageClassName),
		},
	}
}

// OwnedReferencesQuarksStatefulSet for use in tests
func (c *Catalog) OwnedReferencesQuarksStatefulSet(name string) qstsv1a1.QuarksStatefulSet {
	return qstsv1a1.QuarksStatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: qstsv1a1.QuarksStatefulSetSpec{
			UpdateOnConfigChange: true,
			Template:             c.OwnedReferencesStatefulSet(name),
		},
	}
}
