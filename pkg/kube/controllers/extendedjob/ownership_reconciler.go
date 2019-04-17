package extendedjob

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/finalizer"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

var _ reconcile.Reconciler = &OwnershipReconciler{}

// NewOwnershipReconciler returns a new reconciler for errand jobs
func NewOwnershipReconciler(
	ctx context.Context,
	config *config.Config,
	mgr manager.Manager,
	f setOwnerReferenceFunc,
	owner Owner,
) reconcile.Reconciler {
	versionedSecretStore := versionedsecretstore.NewVersionedSecretStore(mgr.GetClient())

	return &OwnershipReconciler{
		ctx:                  ctx,
		client:               mgr.GetClient(),
		config:               config,
		scheme:               mgr.GetScheme(),
		setOwnerReference:    f,
		owner:                owner,
		versionedSecretStore: versionedSecretStore,
	}
}

// OwnershipReconciler implements the Reconciler interface
type OwnershipReconciler struct {
	ctx                  context.Context
	client               client.Client
	config               *config.Config
	scheme               *runtime.Scheme
	setOwnerReference    setOwnerReferenceFunc
	owner                Owner
	versionedSecretStore versionedsecretstore.VersionedSecretStore
}

// Reconcile keeps track of ownership on all configs, configmaps and secrets,
// which are used by extended jobs.  Only called for CRs, which have
// UpdateOnConfigChange set to true.
func (r *OwnershipReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	var result = reconcile.Result{}
	eJob := &ejv1.ExtendedJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling EJob '%s' configs ownership", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, eJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// do not requeue, extended job is probably deleted
			ctxlog.Infof(ctx, "Failed to find EJob '%s', not retrying: %s", request.NamespacedName, err)
			err = nil
			return result, err
		}
		// Error reading the object - requeue the request.
		ctxlog.Errorf(ctx, "Failed to get EJob '%s': %s", request.NamespacedName, err)
		return result, err
	}

	eJobCopy := eJob.DeepCopy()
	err = r.versionedSecretStore.UpdateSecretReferences(ctx, eJob.GetNamespace(), &eJob.Spec.Template.Spec)
	if err != nil {
		ctxlog.Errorf(ctx, "Could not update versioned secrets of EJob '%s' before sync: %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// Remove all ownership from configs and the finalizer from eJob
	if !eJob.Spec.UpdateOnConfigChange || eJob.ToBeDeleted() {
		existingConfigs, err := r.owner.ListConfigsOwnedBy(ctx, eJob)
		if err != nil {
			return result, errors.Wrapf(err, "Could not list ConfigMaps and Secrets owned by '%s'", eJob.Name)
		}
		err = r.owner.RemoveOwnerReferences(ctx, eJob, existingConfigs)
		if err != nil {
			ctxlog.WithEvent(eJob, "RemoveOwnerReferenceError").Errorf(ctx, "Could not remove OwnerReferences pointing to eJob '%s': %s", eJob.Name, err)
			return reconcile.Result{}, err
		}

		finalizer.RemoveFinalizer(eJob)
		err = r.client.Update(ctx, eJob)
		if err != nil {
			ctxlog.WithEvent(eJob, "RemoveFinalizerError").Errorf(ctx, "Could not remove finalizer from EJob '%s': %s", eJob.GetName(), err)
			return reconcile.Result{}, err
		}

		return result, err
	}

	ctxlog.Debugf(ctx, "Updating ownerReferences for EJob '%s' in namespace '%s'", eJob.Name, eJob.Namespace)

	err = r.owner.Sync(ctx, eJob, eJob.Spec.Template.Spec)
	if err != nil {
		return result, fmt.Errorf("error updating OwnerReferences: %s", err)
	}

	if !finalizer.HasFinalizer(eJob) {
		ctxlog.Debugf(ctx, "Add finalizer to EJob '%s' in namespace '%s'", eJob.Name, eJob.Namespace)
		finalizer.AddFinalizer(eJob)
	}

	// Update if needed
	if !reflect.DeepEqual(eJob, eJobCopy) {
		ctxlog.Debugf(ctx, "Updating EJob '%s' in namespace '%s'", eJob.Name, eJob.Namespace)
		err = r.client.Update(ctx, eJob)
		if err != nil {
			ctxlog.WithEvent(eJob, "UpdateEJobError").Errorf(ctx, "Could not update EJob '%s': %s", eJob.GetName(), err)
			return reconcile.Result{}, err
		}
	}
	return result, err
}
