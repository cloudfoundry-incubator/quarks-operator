package operator

import (
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controller"
)

// NewManager adds schemes, controllers and starts the manager
func NewManager(log *zap.SugaredLogger, cfg *rest.Config, options manager.Options) (mgr manager.Manager, err error) {
	mgr, err = manager.New(cfg, options)
	if err != nil {
		return
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err = controller.AddToScheme(mgr.GetScheme()); err != nil {
		return
	}

	// Setup all Controllers
	err = controller.AddToManager(log, mgr)
	return
}
