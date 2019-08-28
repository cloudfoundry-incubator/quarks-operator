package operator

import (
	"context"

	"github.com/pkg/errors"
	extv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	credsgen "code.cloudfoundry.org/cf-operator/pkg/credsgen/in_memory_generator"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	essv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedstatefulset/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/controllers"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/crd"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

// NewManager adds schemes, controllers and starts the manager
func NewManager(ctx context.Context, config *config.Config, cfg *rest.Config, options manager.Options) (manager.Manager, error) {
	mgr, err := manager.New(cfg, options)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize new manager")
	}

	log := ctxlog.ExtractLogger(ctx)

	log.Info("Registering Components")
	config.Namespace = options.Namespace

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
	err = crd.ApplyCRD(
		exClient,
		bdv1.BOSHDeploymentResourceName,
		bdv1.BOSHDeploymentResourceKind,
		bdv1.BOSHDeploymentResourcePlural,
		bdv1.BOSHDeploymentResourceShortNames,
		bdv1.SchemeGroupVersion,
		&bdv1.BOSHDeploymentValidation,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", bdv1.BOSHDeploymentResourceName)
	}
	err = crd.WaitForCRDReady(exClient, bdv1.BOSHDeploymentResourceName)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for CRD '%s' ready", bdv1.BOSHDeploymentResourceName)
	}

	err = crd.ApplyCRD(
		exClient,
		ejv1.ExtendedJobResourceName,
		ejv1.ExtendedJobResourceKind,
		ejv1.ExtendedJobResourcePlural,
		ejv1.ExtendedJobResourceShortNames,
		ejv1.SchemeGroupVersion,
		&ejv1.ExtendedJobValidation,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", ejv1.ExtendedJobResourceName)
	}
	err = crd.WaitForCRDReady(exClient, ejv1.ExtendedJobResourceName)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for CRD '%s' ready", ejv1.ExtendedJobResourceName)
	}

	err = crd.ApplyCRD(
		exClient,
		esv1.ExtendedSecretResourceName,
		esv1.ExtendedSecretResourceKind,
		esv1.ExtendedSecretResourcePlural,
		esv1.ExtendedSecretResourceShortNames,
		esv1.SchemeGroupVersion,
		nil,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", esv1.ExtendedSecretResourceName)
	}
	err = crd.WaitForCRDReady(exClient, esv1.ExtendedSecretResourceName)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for CRD '%s' ready", esv1.ExtendedSecretResourceName)
	}

	err = crd.ApplyCRD(
		exClient,
		essv1.ExtendedStatefulSetResourceName,
		essv1.ExtendedStatefulSetResourceKind,
		essv1.ExtendedStatefulSetResourcePlural,
		essv1.ExtendedStatefulSetResourceShortNames,
		essv1.SchemeGroupVersion,
		&essv1.ExtendedJobValidation,
	)
	if err != nil {
		return errors.Wrapf(err, "failed to apply CRD '%s'", essv1.ExtendedStatefulSetResourceName)
	}
	return crd.WaitForCRDReady(exClient, essv1.ExtendedStatefulSetResourceName)
}
