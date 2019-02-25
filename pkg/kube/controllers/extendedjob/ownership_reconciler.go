package extendedjob

import (
	"fmt"

	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/finalizer"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ reconcile.Reconciler = &OwnershipReconciler{}

// NewOwnershipReconciler returns a new reconciler for errand jobs
func NewOwnershipReconciler(
	log *zap.SugaredLogger,
	config *context.Config,
	mgr manager.Manager,
	f setOwnerReferenceFunc,
	owner Owner,
) reconcile.Reconciler {
	log.Info("Creating a reconciler for errand ExtendedJobs")

	return &OwnershipReconciler{
		client:            mgr.GetClient(),
		log:               log,
		config:            config,
		scheme:            mgr.GetScheme(),
		setOwnerReference: f,
		owner:             owner,
	}
}

// OwnershipReconciler implements the Reconciler interface
type OwnershipReconciler struct {
	client            client.Client
	log               *zap.SugaredLogger
	config            *context.Config
	scheme            *runtime.Scheme
	setOwnerReference setOwnerReferenceFunc
	owner             Owner
}

// Reconcile keeps track of ownership on all configs, configmaps and secrets,
// which are used by extended jobs.  Only called for CRs, which have
// UpdateOnConfigChange set to true.
func (r *OwnershipReconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log.Info("Reconciling errand job configs ownership ", request.NamespacedName)

	var result = reconcile.Result{}
	extJob := &ejv1.ExtendedJob{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.NewBackgroundContextWithTimeout(r.config.CtxType, r.config.CtxTimeOut)
	defer cancel()

	err := r.client.Get(ctx, request.NamespacedName, extJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// do not requeue, extended job is probably deleted
			r.log.Infof("Failed to find extended job '%s', not retrying: %s", request.NamespacedName, err)
			err = nil
			return result, err
		}
		// Error reading the object - requeue the request.
		r.log.Errorf("Failed to get the extended job '%s': %s", request.NamespacedName, err)
		return result, err
	}

	// Remove all ownership from configs and the finalizer from extJob
	if !extJob.Spec.UpdateOnConfigChange || extJob.ToBeDeleted() {
		existingConfigs, err := r.owner.ListConfigsOwnedBy(ctx, extJob)
		if err != nil {
			return result, errors.Wrapf(err, "Could not list ConfigMaps and Secrets owned by '%s'", extJob.Name)
		}
		err = r.owner.RemoveOwnerReferences(ctx, extJob, existingConfigs)
		if err != nil {
			r.log.Errorf("Could not remove OwnerReferences pointing to extJob '%s': %s", extJob.Name, err)
			return reconcile.Result{}, err
		}

		finalizer.RemoveFinalizer(extJob)
		err = r.client.Update(ctx, extJob)
		if err != nil {
			r.log.Errorf("Could not remove finalizer from ExtJob '%s': ", extJob.GetName(), err)
			return reconcile.Result{}, err
		}

		return result, err
	}

	r.log.Debugf("Updating ownerReferences for ExtendedJob '%s' in namespace '%s'.", extJob.Name, extJob.Namespace)

	err = r.owner.Sync(ctx, extJob, extJob.Spec.Template.Spec)
	if err != nil {
		return result, fmt.Errorf("error updating OwnerReferences: %s", err)
	}

	if !finalizer.HasFinalizer(extJob) {
		r.log.Debugf("Add finalizer to extendedJob '%s' in namespace '%s'.", extJob.Name, extJob.Namespace)
		finalizer.AddFinalizer(extJob)
		err = r.client.Update(ctx, extJob)
		if err != nil {
			r.log.Errorf("Could not remove finalizer from ExtJob '%s': ", extJob.GetName(), err)
			return reconcile.Result{}, err
		}

	}
	return result, err
}
