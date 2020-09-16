package operator

import (
	"context"

	"github.com/pkg/errors"

	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	extv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	bdv1 "code.cloudfoundry.org/quarks-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	"code.cloudfoundry.org/quarks-operator/pkg/kube/controllers"
	qstsv1a1 "code.cloudfoundry.org/quarks-statefulset/pkg/kube/apis/quarksstatefulset/v1alpha1"
	"code.cloudfoundry.org/quarks-utils/pkg/config"
	"code.cloudfoundry.org/quarks-utils/pkg/crd"
	credsgen "code.cloudfoundry.org/quarks-utils/pkg/credsgen/in_memory_generator"
	"code.cloudfoundry.org/quarks-utils/pkg/ctxlog"
)

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

// ApplyCRDs applies a bdpl CRD into the cluster
func ApplyCRDs(ctx context.Context, config *rest.Config) error {
	client, err := extv1client.NewForConfig(config)
	if err != nil {
		return errors.Wrap(err, "Could not get kube client")
	}

	// Add bdpl crd
	b := crd.New(
		bdv1.BOSHDeploymentResourceName,
		extv1.CustomResourceDefinitionNames{
			Kind:       bdv1.BOSHDeploymentResourceKind,
			Plural:     bdv1.BOSHDeploymentResourcePlural,
			ShortNames: bdv1.BOSHDeploymentResourceShortNames,
		},
		bdv1.SchemeGroupVersion,
	)

	err = b.WithValidation(&bdv1.BOSHDeploymentValidation).
		WithAdditionalPrinterColumns(bdv1.BOSHDeploymentAdditionalPrinterColumns).
		Build().
		Apply(ctx, client)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", bdv1.BOSHDeploymentResourceName)
	}
	err = crd.WaitForCRDReady(ctx, client, bdv1.BOSHDeploymentResourceName)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for CRD '%s' ready", bdv1.BOSHDeploymentResourceName)
	}

	// Add qsts crd
	q := crd.New(
		qstsv1a1.QuarksStatefulSetResourceName,
		extv1.CustomResourceDefinitionNames{
			Kind:       qstsv1a1.QuarksStatefulSetResourceKind,
			Plural:     qstsv1a1.QuarksStatefulSetResourcePlural,
			ShortNames: qstsv1a1.QuarksStatefulSetResourceShortNames,
		},
		qstsv1a1.SchemeGroupVersion,
	)

	err = q.WithValidation(&qstsv1a1.QuarksStatefulSetValidation).
		WithAdditionalPrinterColumns(qstsv1a1.QuarksStatefulSetAdditionalPrinterColumns).
		Build().
		Apply(ctx, client)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", qstsv1a1.QuarksStatefulSetResourceName)
	}
	err = crd.WaitForCRDReady(ctx, client, qstsv1a1.QuarksStatefulSetResourceName)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for CRD '%s' ready", qstsv1a1.QuarksStatefulSetResourceName)
	}
	return nil
}
