package operator

import (
	"context"

	"github.com/pkg/errors"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	extv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	credsgen "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	qsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarkssecret/v1alpha1"
	qstsv1a1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	qjv1a1 "code.cloudfoundry.org/quarks-job/pkg/kube/apis/quarksjob/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/crd"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

type resource struct {
	name         string
	kind         string
	plural       string
	shortNames   []string
	groupVersion schema.GroupVersion
	validation   *extv1.CustomResourceValidation
}

// NewManager adds schemes, controllers and starts the manager
func NewManager(ctx context.Context, config *config.Config, cfg *rest.Config, options manager.Options) (manager.Manager, error) {
	mgr, err := manager.New(cfg, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize new manager")
	}

	log := ctxlog.ExtractLogger(ctx)

	log.Info("Registering Components")

	// Setup Scheme for all resources
	err = controllers.AddToScheme(mgr.GetScheme())
	if err != nil {
		return nil, errors.Wrap(err, "failed to add manager scheme to controllers")
	}

	// Setup Hooks for all resources
	err = controllers.AddHooks(ctx, config, mgr, credsgen.NewInMemoryGenerator(log))
	if err != nil {
		return nil, errors.Wrap(err, "failed to setup hooks")
	}

	// Setup all Controllers
	err = controllers.AddToManager(ctx, config, mgr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to add controllers to manager")
	}

	return mgr, nil
}

// ApplyCRDs applies a collection of CRDs into the cluster
func ApplyCRDs(config *rest.Config) error {
	exClient, err := extv1client.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}

	for _, res := range []resource{
		{
			bdv1.BOSHDeploymentResourceName,
			bdv1.BOSHDeploymentResourceKind,
			bdv1.BOSHDeploymentResourcePlural,
			bdv1.BOSHDeploymentResourceShortNames,
			bdv1.SchemeGroupVersion,
			&bdv1.BOSHDeploymentValidation,
		},
		{
			qjv1a1.QuarksJobResourceName,
			qjv1a1.QuarksJobResourceKind,
			qjv1a1.QuarksJobResourcePlural,
			qjv1a1.QuarksJobResourceShortNames,
			qjv1a1.SchemeGroupVersion,
			&qjv1a1.QuarksJobValidation,
		},
		{
			qsv1a1.QuarksSecretResourceName,
			qsv1a1.QuarksSecretResourceKind,
			qsv1a1.QuarksSecretResourcePlural,
			qsv1a1.QuarksSecretResourceShortNames,
			qsv1a1.SchemeGroupVersion,
			&qsv1a1.QuarksSecretValidation,
		},
		{
			qstsv1a1.QuarksStatefulSetResourceName,
			qstsv1a1.QuarksStatefulSetResourceKind,
			qstsv1a1.QuarksStatefulSetResourcePlural,
			qstsv1a1.QuarksStatefulSetResourceShortNames,
			qstsv1a1.SchemeGroupVersion,
			&qstsv1a1.QuarksStatefulSetValidation,
		},
	} {
		err = crd.ApplyCRD(
			exClient,
			res.name,
			res.kind,
			res.plural,
			res.shortNames,
			res.groupVersion,
			res.validation,
		)
		if err != nil {
			return errors.Wrapf(err, "failed to apply CRD '%s'", res.name)
		}
		err = crd.WaitForCRDReady(exClient, res.name)
		if err != nil {
			return errors.Wrapf(err, "failed to wait for CRD '%s' ready", res.name)
		}
	}

	return nil
}
