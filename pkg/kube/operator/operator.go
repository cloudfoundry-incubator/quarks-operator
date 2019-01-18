package operator

import (
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"github.com/pkg/errors"
)

// NewManager adds schemes, controllers and starts the manager
func NewManager(log *zap.SugaredLogger, cfg *rest.Config, options manager.Options) (mgr manager.Manager, err error) {
	mgr, err = manager.New(cfg, options)
	if err != nil {
		return
	}

	log.Info("Registering Components.")

	// Setup Scheme for all resources
	if err = controllers.AddToScheme(mgr.GetScheme()); err != nil {
		return
	}

	// Setup indexer
	ev := &corev1.Event{}
	err = mgr.GetCache().IndexField(ev, "involvedObject.kind", func(obj runtime.Object) []string {
		return []string{string(obj.(*corev1.Event).InvolvedObject.Kind)}
	})
	if err != nil {
		err = errors.Wrap(err, "failed to add indexer to cache")
		return
	}

	// Setup all Controllers
	err = controllers.AddToManager(log, mgr)
	return
}
