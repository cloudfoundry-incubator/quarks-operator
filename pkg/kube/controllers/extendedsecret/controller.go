package extendedsecret

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	credsgen "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	es "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// Add creates a new ExtendedSecrets Controller and adds it to the Manager
func Add(ctx context.Context, config *config.Config, mgr manager.Manager) error {
	ctx = ctxlog.NewReconcilerContext(ctx, "ext-secret-reconciler", mgr.GetRecorder("ext-secret-recorder"))
	log := ctxlog.ExtractLogger(ctx)
	r := NewReconciler(ctx, config, mgr, credsgen.NewInMemoryGenerator(log), controllerutil.SetControllerReference)

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
