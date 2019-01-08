package extendedsecret

import (
	"go.uber.org/zap"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	credsgen "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	es "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
)

// Add creates a new ExtendedSecrets Controller and adds it to the Manager
func Add(log *zap.SugaredLogger, mgr manager.Manager) error {
	r := NewReconciler(log, mgr, credsgen.NewInMemoryGenerator(log))

	// Create a new controller
	c, err := controller.New("extendedsecret-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to ExtendedSecrets
	err = c.Watch(&source.Kind{Type: &es.ExtendedSecret{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}
