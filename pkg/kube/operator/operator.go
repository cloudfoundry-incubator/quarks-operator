package operator

import (
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
<<<<<<< HEAD
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
=======
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllersconfig"
>>>>>>> a028cb472ee656cbc66f581a8a279dcd4a458f61
)

// NewManager adds schemes, controllers and starts the manager
func NewManager(log *zap.SugaredLogger, ctrConfig *context.Config, cfg *rest.Config, options manager.Options) (mgr manager.Manager, err error) {
	mgr, err = manager.New(cfg, options)
	if err != nil {
		return
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err = controllers.AddToScheme(mgr.GetScheme()); err != nil {
		return
	}

	// Setup Hooks for all resources
	if err = controllers.AddHooks(log, ctrConfig, mgr); err != nil {
		return
	}

	// Setup all Controllers
	err = controllers.AddToManager(log, ctrConfig, mgr)
	return
}
