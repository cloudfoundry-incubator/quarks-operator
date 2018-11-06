package apis

import (
	"code.cloudfoundry.org/cf-operator/pkg/apis/fissile/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
)

var addToSchemes = runtime.SchemeBuilder{
	v1alpha1.SchemeBuilder.AddToScheme,
}

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return addToSchemes.AddToScheme(s)
}
