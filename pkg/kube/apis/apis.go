package apis

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"code.cloudfoundry.org/quarks-utils/pkg/names"
)

const (
	// GroupName defines the kube group name for all controllers
	// It's what's used when you specify the apiVersion of a resource.
	// e.g.:
	//
	//   ---
	//   apiVersion: quarks.cloudfoundry.org/v1alpha1
	//   kind: BOSHDeployment
	//   ...
	GroupName = names.GroupName
)

// Object is used as a helper interface when passing Kubernetes resources
// between methods.
// All Kubernetes resources should implement both of these interfaces
type Object interface {
	runtime.Object
	metav1.Object
}
