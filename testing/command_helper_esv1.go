package testing

import (
	"io/ioutil"

	"github.com/pkg/errors"

	"sigs.k8s.io/yaml"

	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util"
)

// AddTestStorageClassToVolumeClaimTemplates adds storage class to the example and returns the new file temporary path
func AddTestStorageClassToVolumeClaimTemplates(filePath string, class string) (string, error) {

	extendedStatefulSet := essv1.ExtendedStatefulSet{}
	extendedStatefulSetBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", errors.Wrapf(err, "Reading file %s failed.", filePath)
	}
	err = yaml.Unmarshal(extendedStatefulSetBytes, &extendedStatefulSet)
	if err != nil {
		return "", errors.Wrapf(err, "Unmarshalling extendedstatefulset from file %s failed.", filePath)
	}

	if extendedStatefulSet.Spec.Template.Spec.VolumeClaimTemplates != nil {
		volumeClaimTemplates := extendedStatefulSet.Spec.Template.Spec.VolumeClaimTemplates
		for volumeClaimTemplateIndex := range volumeClaimTemplates {
			volumeClaimTemplates[volumeClaimTemplateIndex].Spec.StorageClassName = util.String(class)
		}
		extendedStatefulSet.Spec.Template.Spec.VolumeClaimTemplates = volumeClaimTemplates
	} else {
		return "", errors.Errorf("No volumeclaimtemplates present in the %s yaml", filePath)
	}

	extendedStatefulSetBytes, err = yaml.Marshal(&extendedStatefulSet)
	if err != nil {
		return "", errors.Wrapf(err, "Marshing extendedstatfulset %s failed", extendedStatefulSet.GetName())
	}

	tmpFilePath := "/tmp/example.yaml"

	err = ioutil.WriteFile(tmpFilePath, extendedStatefulSetBytes, 0644)
	if err != nil {
		return "", errors.Wrapf(err, "Writing extendedstatefulset %s to file %s failed.", extendedStatefulSet.GetName(), tmpFilePath)
	}

	return tmpFilePath, nil
}
