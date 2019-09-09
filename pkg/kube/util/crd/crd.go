package crd

import (
	"reflect"
	"time"

	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	extv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
)

// ApplyCRD creates or updates the CRD
func ApplyCRD(client extv1client.ApiextensionsV1beta1Interface, crdName, kind, plural string, shortNames []string, groupVersion schema.GroupVersion, validation *extv1.CustomResourceValidation) error {
	crd := &extv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crdName,
		},
		Spec: extv1.CustomResourceDefinitionSpec{
			Group: groupVersion.Group,
			Versions: []extv1.CustomResourceDefinitionVersion{
				{
					Name:    groupVersion.Version,
					Served:  true,
					Storage: true,
				},
			},
			Subresources: &extv1.CustomResourceSubresources{
				Status: &extv1.CustomResourceSubresourceStatus{},
			},
			Scope: extv1.NamespaceScoped,
			Names: extv1.CustomResourceDefinitionNames{
				Kind:       kind,
				Plural:     plural,
				ShortNames: shortNames,
			},
		},
	}

	if validation != nil {
		crd.Spec.Validation = validation
	}

	exCrd, err := client.CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "getting CRD '%s'", crdName)
		}
		_, err := client.CustomResourceDefinitions().Create(crd)
		if err != nil {
			return errors.Wrapf(err, "creating CRD '%s'", crdName)
		}
		return nil
	}

	if !reflect.DeepEqual(crd.Spec, exCrd.Spec) {
		crd.ResourceVersion = exCrd.ResourceVersion
		_, err = client.CustomResourceDefinitions().Update(crd)
		if err != nil {
			return errors.Wrapf(err, "updating CRD '%s'", crdName)
		}
	}

	return nil
}

// WaitForCRDReady blocks until the CRD is ready.
func WaitForCRDReady(client extv1client.ApiextensionsV1beta1Interface, crdName string) error {
	err := wait.ExponentialBackoff(
		wait.Backoff{
			Duration: time.Second,
			Steps:    15,
			Factor:   1,
		},
		func() (bool, error) {
			crd, err := client.CustomResourceDefinitions().Get(crdName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			for _, cond := range crd.Status.Conditions {
				if cond.Type == extv1.NamesAccepted && cond.Status == extv1.ConditionTrue {
					return true, nil
				}
			}

			return false, nil
		})
	if err != nil {
		return errors.Wrapf(err, "Waiting for CRD ready failed")
	}

	return nil
}
