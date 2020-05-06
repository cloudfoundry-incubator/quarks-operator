package monitorednamespace

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"code.cloudfoundry.org/quarks-operator/pkg/kube/apis"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

var (
	// LabelNamespace key for label on a namespace to indicate that cf-operator is monitoring it.
	// Can be used as an ID, to keep operators in a cluster from intefering with each other.
	LabelNamespace = fmt.Sprintf("%s/monitored", apis.GroupName)
)

// monitored retrieves the namespace and checks the monitored label for id
func monitored(ctx context.Context, client client.Client, name string, id string) bool {
	ns := &corev1.Namespace{}
	if err := client.Get(ctx, types.NamespacedName{Name: name}, ns); err != nil {
		ctxlog.Errorf(ctx, "failed to get namespaces '%s'", name)
		return false
	}
	if value, ok := ns.Labels[LabelNamespace]; ok && value == id {
		return true
	}
	return false
}

// NewNSPredicate returns a controller predicate to filter for namespaces with a label
func NewNSPredicate(ctx context.Context, client client.Client, id string) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			name := e.Meta.GetNamespace()
			return monitored(ctx, client, name, id)
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			name := e.MetaNew.GetNamespace()
			return monitored(ctx, client, name, id)
		},
	}
}
