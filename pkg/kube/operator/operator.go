package operator

import (
	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"code.cloudfoundry.org/cf-operator/version"
)

var (
	// DockerOrganization is the organization which provides the operator image
	DockerOrganization = ""
	// DockerRepository is the repository which provides the operator image
	DockerRepository = ""
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

	// Setup all Controllers
	err = controllers.AddToManager(log, ctrConfig, mgr)
	return
}

// GetOperatorDockerImage returns the image name of the operator docker image
func GetOperatorDockerImage() string {
	return DockerOrganization + "/" + DockerRepository + ":" + version.Version
}
