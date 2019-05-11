package boshdeployment

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
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

	log.Infof(ctx, "Reconciling BOSHDeployment %s", request.NamespacedName)
	instance := &bdv1.BOSHDeployment{}
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: BOSHDeployment not found")
			return reconcile.Result{}, nil
		}
		err = log.WithEvent(instance, "GetBOSHDeploymentError").Errorf(ctx, "Failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Get state from instance
	instanceState := instance.Status.State

	// Resolve the manifest (incl. ops files and implicit variables)
	manifest, err := r.resolveManifest(ctx, instance)
	if err != nil {
		log.WithEvent(instance, "BadManifestError").Infof(ctx, "Error resolving the manifest %s", instance.GetName())
		return reconcile.Result{}, errors.Wrap(err, "could not resolve manifest")
	}

	// Generate all the kube objects we need for the manifest
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	kubeConfigs, err := manifest.ConvertToKube(r.config.Namespace)
	if err != nil {
		err = log.WithEvent(instance, "BadManifestError").Errorf(ctx, "Error converting bosh manifest %s to kube objects: %s", manifest.Name, err)
		return reconcile.Result{}, err
	}

	switch instanceState {
	case OpsAppliedState:
		err = r.generateVariableSecrets(ctx, instance, &kubeConfigs)
		if err != nil {
			err = log.WithEvent(instance, "VariableGenerationError").Errorf(ctx, "Failed to generate variables: %v", err)
			return reconcile.Result{}, err
		}
	default:
		return reconcile.Result{}, nil
	}

	log.Debugf(ctx, "Requeue the reconcile: BoshDeployment '%s/%s' is in state '%s'", instance.GetNamespace(), instance.GetName(), instance.Status.State)
	return reconcile.Result{Requeue: true}, r.updateInstanceState(ctx, instance)
}

// resolveManifest resolves the manifest and applies ops files and implicit variable interpolation
func (r *ReconcileGeneratedVariable) resolveManifest(ctx context.Context, instance *bdv1.BOSHDeployment) (*bdm.Manifest, error) {
	// Create temp manifest as variable interpolation job input
	// retrieve manifest
	log.Debug(ctx, "Resolving manifest")
	manifest, err := r.resolver.ResolveManifest(instance, instance.GetNamespace())
	if err != nil {
		log.WithEvent(instance, "ResolveManifestError").Errorf(ctx, "Error resolving the manifest %s: %s", instance.GetName(), err)
		return nil, err
	}

	// Replace the name with the name of the BOSHDeployment resource
	manifest.Name = instance.GetName()

	return manifest, nil
}

// generateVariableSecrets create variables extendedSecrets
func (r *ReconcileGeneratedVariable) generateVariableSecrets(ctx context.Context, instance *bdv1.BOSHDeployment, kubeConfig *bdm.KubeConfig) error {
	log.Debug(ctx, "Creating variables extendedSecrets")
	var err error
	for _, variable := range kubeConfig.Variables {
		// Set BOSHDeployment instance as the owner and controller
		if err := r.setReference(instance, &variable, r.scheme); err != nil {
			return errors.Wrap(err, "could not set reference for an ExtendedStatefulSet for a BOSH Deployment")
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

	instance.Status.State = VariableGeneratedState

	return nil
}

// updateInstanceState update instance state
func (r *ReconcileGeneratedVariable) updateInstanceState(ctx context.Context, currentInstance *bdv1.BOSHDeployment) error {
	currentManifestSHA1, _ := currentInstance.GetAnnotations()[bdv1.AnnotationManifestSHA1]

	// Fetch latest BOSHDeployment before update
	foundInstance := &bdv1.BOSHDeployment{}
	key := types.NamespacedName{Namespace: currentInstance.GetNamespace(), Name: currentInstance.GetName()}
	err := r.client.Get(ctx, key, foundInstance)
	if err != nil {
		log.Errorf(ctx, "Failed to get BOSHDeployment instance '%s': %v", currentInstance.GetName(), err)
		return errors.Wrapf(err, "could not get BOSHDeployment instance '%s' when update instance state", currentInstance.GetName())
	}
	oldManifestSHA1, _ := foundInstance.GetAnnotations()[bdv1.AnnotationManifestSHA1]

	if oldManifestSHA1 != currentManifestSHA1 {
		// Set manifest SHA1
		if foundInstance.Annotations == nil {
			foundInstance.Annotations = map[string]string{}
		}

		foundInstance.Annotations[bdv1.AnnotationManifestSHA1] = currentManifestSHA1
	}

	// Update the Status of the resource
	if !reflect.DeepEqual(foundInstance.Status.State, currentInstance.Status.State) {
		log.Debugf(ctx, "Updating boshDeployment from '%s' to '%s'", foundInstance.Status.State, currentInstance.Status.State)

		newInstance := foundInstance.DeepCopy()
		newInstance.Status.State = currentInstance.Status.State

		err = r.client.Update(ctx, newInstance)
		if err != nil {
			log.Errorf(ctx, "Failed to update BOSHDeployment instance status: %v", err)
			return errors.Wrapf(err, "could not update BOSHDeployment instance '%s' when update instance state", currentInstance.GetName())
		}
	}

	return nil
}
