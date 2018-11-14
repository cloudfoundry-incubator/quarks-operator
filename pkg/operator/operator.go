package operator

import (
	"code.cloudfoundry.org/cf-operator/pkg/apis"
	"code.cloudfoundry.org/cf-operator/pkg/controller"
	"go.uber.org/zap"

	// if running on GKE
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NewManager adds schemes, controllers and starts the manager
func NewManager(log *zap.SugaredLogger, cfg *rest.Config, options manager.Options) (mgr manager.Manager, err error) {
	mgr, err = manager.New(cfg, options)
	if err != nil {
		return
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err = apis.AddToScheme(mgr.GetScheme()); err != nil {
		return
	}

	// Setup all Controllers
	err = controller.AddToManager(log, mgr)
	return
}
