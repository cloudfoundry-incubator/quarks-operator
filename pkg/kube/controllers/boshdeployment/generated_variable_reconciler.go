package boshdeployment

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	esv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedsecret/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
)

var _ reconcile.Reconciler = &ReconcileGeneratedVariable{}

// NewGeneratedVariableReconciler returns a new reconcile.Reconciler
func NewGeneratedVariableReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, srf setReferenceFunc, kubeConverter *converter.KubeConverter) reconcile.Reconciler {
	return &ReconcileGeneratedVariable{
		ctx:           ctx,
		config:        config,
		client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		setReference:  srf,
		kubeConverter: kubeConverter,
	}
}

// ReconcileGeneratedVariable reconciles a manifest with ops
type ReconcileGeneratedVariable struct {
	ctx           context.Context
	config        *config.Config
	client        client.Client
	scheme        *runtime.Scheme
	setReference  setReferenceFunc
	kubeConverter *converter.KubeConverter
}

// Reconcile creates or updates variables extendedSecrets
func (r *ReconcileGeneratedVariable) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()
	log.Infof(ctx, "Reconciling ops applied manifest secret '%s'", request.NamespacedName)
	manifestSecret := &corev1.Secret{}
	err := r.client.Get(ctx, request.NamespacedName, manifestSecret)

	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: manifest with ops file secret not found")
			return reconcile.Result{}, nil
		}

		err = log.WithEvent(manifestSecret, "GetBOSHDeploymentManifestWithOpsFileError").Errorf(ctx, "Failed to get BOSHDeployment manifest with ops file secret '%s': %v", request.NamespacedName, err)
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	var manifestContents string

	// Get the manifest yaml
	if val, ok := manifestSecret.Data["manifest.yaml"]; ok {
		manifestContents = string(val)
	} else {
		return reconcile.Result{}, errors.New("Couldn't find manifest.yaml key in manifest secret")
	}

	// Unmarshal the manifest
	log.Debug(ctx, "Unmarshaling BOSHDeployment manifest from manifest with ops secret")
	manifest, err := bdm.LoadYAML([]byte(manifestContents))
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(manifestSecret, "BadManifestError").Errorf(ctx, "Failed to unmarshal manifest from secret '%s': %v", request.NamespacedName, err)

	}

	// Convert the manifest to kube objects
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	secrets := r.kubeConverter.Variables(manifest.Name, manifest.Variables)

	if len(secrets) == 0 {
		log.Debug(ctx, "Skip generate variable extendedSecrets: there are no variables")
		return reconcile.Result{}, nil
	}

	// Create/update all explicit BOSH Variables
	err = r.generateVariableSecrets(ctx, manifestSecret, secrets)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(manifestSecret, "VariableGenerationError").Errorf(ctx, "Failed to generate variables for bosh manifest '%s': %v", manifest.Name, err)
	}

	return reconcile.Result{}, nil
}

// generateVariableSecrets create variables extendedSecrets
func (r *ReconcileGeneratedVariable) generateVariableSecrets(ctx context.Context, manifestSecret *corev1.Secret, variables []esv1.ExtendedSecret) error {
	log.Debug(ctx, "Creating ExtendedSecrets for explicit variables")
	for _, variable := range variables {
		// Set the "manifest with ops" secret as the owner for the ExtendedSecrets
		// The "manifest with ops" secret is owned by the actual BOSHDeployment, so everything
		// should be garbage collected properly.

		if err := r.setReference(manifestSecret, &variable, r.scheme); err != nil {
			err = log.WithEvent(manifestSecret, "OwnershipError").Errorf(ctx, "Failed to set ownership for %s: %v", variable.Name, err)
			return err
		}

		op, err := controllerutil.CreateOrUpdate(ctx, r.client, variable.DeepCopy(), func(obj runtime.Object) error {
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

		log.Debugf(ctx, "ExtendedSecret '%s' has been %s", variable.Name, op)
	}

	return nil
}
