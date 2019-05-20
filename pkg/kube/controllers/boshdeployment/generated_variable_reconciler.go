package boshdeployment

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

var _ reconcile.Reconciler = &ReconcileGeneratedVariable{}

// NewGeneratedVariableReconciler returns a new reconcile.Reconciler
func NewGeneratedVariableReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver bdm.Resolver, srf setReferenceFunc) reconcile.Reconciler {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &ReconcileGeneratedVariable{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		scheme:               mgr.GetScheme(),
		resolver:             resolver,
		setReference:         srf,
		owner:                owner.NewOwner(mgr.GetClient(), mgr.GetScheme()),
		versionedSecretStore: versionedSecretStore,
	}
}

// ReconcileGeneratedVariable reconciles a BOSHDeployment object
type ReconcileGeneratedVariable struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	ctx                  context.Context
	client               client.Client
	scheme               *runtime.Scheme
	resolver             bdm.Resolver
	setReference         setReferenceFunc
	config               *config.Config
	owner                Owner
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// Reconcile reads that state of the cluster for a BOSHDeployment object and makes changes based on the state read
// and what is in the BOSHDeployment.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileGeneratedVariable) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()
	log.Infof(ctx, "Reconciling BOSHDeployment manifest with ops file applied from secret '%s'", request.NamespacedName)
	instance := &corev1.Secret{}
	err := r.client.Get(ctx, request.NamespacedName, instance)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: manifest with ops file secret not found")
			return reconcile.Result{}, nil
		}

		err = log.WithEvent(instance, "GetBOSHDeploymentManifestWithOpsFileError").Errorf(ctx, "Failed to get BOSHDeployment manifest with ops file secret '%s': %v", request.NamespacedName, err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Get the manifest yaml
	manifestContents := instance.StringData["manifest.yaml"]

	// Unmarshal the manifest
	log.Debug(ctx, "Unmarshaling BOSHDeployment manifest from manifest with ops secret")
	manifest := &bdm.Manifest{}
	err = yaml.Unmarshal([]byte(manifestContents), manifest)
	if err != nil {
		err = log.WithEvent(instance, "BadManifestError").Errorf(ctx, "Failed to unmarshal manifest from secret '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// Convert the manifest to kube objects
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	kubeConfigs := bdm.NewKubeConfig(r.config.Namespace, manifest)
	err = kubeConfigs.Convert(*manifest)
	if err != nil {
		err = log.WithEvent(instance, "ManifestConversionError").Errorf(ctx, "Failed to convert bosh manifest '%s' to kube objects: %s", manifest.Name, err)
		return reconcile.Result{}, err
	}

	// Create/update all explicit BOSH Variables
	err = r.generateVariableSecrets(ctx, instance, kubeConfigs)
	if err != nil {
		err = log.WithEvent(instance, "BadManifestError").Errorf(ctx, "Failed to generate variables for bosh manifest '%s': %v", manifest.Name, err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// generateVariableSecrets create variables extendedSecrets
func (r *ReconcileGeneratedVariable) generateVariableSecrets(ctx context.Context, instance *corev1.Secret, kubeConfig *bdm.KubeConfig) error {
	log.Debug(ctx, "Creating ExtendedSecrets for explicit variables")
	var err error
	for _, variable := range kubeConfig.Variables {
		// Set the "manifest with ops" secret as the owner for the ExtendedSecrets
		// The "manifest with ops" secret is owned by the actual BOSHDeployment, so everything
		// should be garbage collected properly.

		if err := r.setReference(instance, &variable, r.scheme); err != nil {
			err = log.WithEvent(instance, "OwnershipError").Errorf(ctx, "Failed to set ownership for %s: %v", variable.Name, err)
			return err
		}

		_, err = controllerutil.CreateOrUpdate(ctx, r.client, variable.DeepCopy(), func(obj runtime.Object) error {
			s, ok := obj.(*esv1.ExtendedSecret)
			if !ok {
				return fmt.Errorf("object is not an ExtendedSecret")
			}
			s.Spec = variable.Spec
			return nil
		})
		if err != nil {
			return errors.Wrapf(err, "creating or updating ExtendedSecret '%s'", variable.Name)
		}
	}

	return nil
}
