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
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	store "code.cloudfoundry.org/cf-operator/pkg/kube/util/store/manifest"
)

// NewManifestReconciler returns a new Reconciler for manifest secret creation
func NewManifestReconciler(ctx context.Context, config *config.Config, mgr manager.Manager) reconcile.Reconciler {
	manifestStore := store.NewStore(mgr.GetClient(), bdm.DesiredManifestKeyName)

	return &ReconcileManifest{
		ctx:           ctx,
		client:        mgr.GetClient(),
		config:        config,
		scheme:        mgr.GetScheme(),
		manifestStore: manifestStore,
	}
}

// ReconcileManifest reconciles an Job object
type ReconcileManifest struct {
	ctx           context.Context
	client        client.Client
	config        *config.Config
	scheme        *runtime.Scheme
	manifestStore store.Store
}

// Reconcile reads that state of the manifest secret
func (r *ReconcileManifest) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	ctxlog.Infof(ctx, "Reconciling BOSHDeployment %s for desired manifest creation", request.NamespacedName)

	// Fetch the BOSHDeployment instance
	instance := &bdv1.BOSHDeployment{}

	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			ctxlog.Debug(ctx, "Skip reconcile: BOSHDeployment not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		ctxlog.Errorf(ctx, "Failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, errors.Wrapf(err, "could not get BOSHDeployment '%s'", request.NamespacedName)
	}

	// Find data gathering exJob
	exJobKey := types.NamespacedName{Name: fmt.Sprintf("data-gathering-%s", instance.GetName()), Namespace: instance.GetNamespace()}
	dataGatheringJob := &ejv1.ExtendedJob{}

	err = r.client.Get(ctx, exJobKey, dataGatheringJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			ctxlog.Debug(ctx, "Reconcile: data gathering job not found")
			return reconcile.Result{Requeue: true}, errors.Wrapf(err, "data gathering job '%s' not found", exJobKey.Name)
		}
		ctxlog.Errorf(ctx, "Failed to get data gathering job '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, errors.Wrapf(err, "could not get data gathering job '%s'", exJobKey.Name)
	}

	// Update volumes for desired manifest secret
	_, interpolatedManifestSecretName := bdm.CalculateEJobOutputSecretPrefixAndName(bdm.DeploymentSecretTypeManifestAndVars, instance.GetName(), bdm.VarInterpolationContainerName)

	ctxlog.Debugf(ctx, "Updating manifest secret to ExtendedJob '%s/%s' volumes", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName())
	err = r.updateExtendedJobVolumes(ctx, instance.GetNamespace(), dataGatheringJob, interpolatedManifestSecretName)
	if err != nil {
		ctxlog.Errorf(ctx, "Failed to update ExtendedJob '%s/%s' Volumes: %v", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName(), err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileManifest) updateExtendedJobVolumes(ctx context.Context, namespace string, dataGatheringJob *ejv1.ExtendedJob, interpolatedManifestSecretName string) error {
	dataGatheringJobCopy := dataGatheringJob.DeepCopy()

	volumes := dataGatheringJob.Spec.Template.Spec.Volumes
	for idx, vol := range volumes {
		if vol.VolumeSource.Secret != nil && vol.VolumeSource.Secret.SecretName == interpolatedManifestSecretName {
			manifestSecret, err := r.manifestStore.LatestSecret(ctx, namespace, vol.VolumeSource.Secret.SecretName, dataGatheringJob.Spec.Output.SecretLabels)
			if err != nil {
				ctxlog.Errorf(ctx, "Could not to get manifest secret for '%s': %v", interpolatedManifestSecretName, err)
				return errors.Wrapf(err, "could not to get manifest secret for '%s'", interpolatedManifestSecretName)
			}

			ctxlog.Debugf(ctx, "Changing manifest secret from '%s' to '%s'", vol.VolumeSource.Secret.SecretName, manifestSecret.GetName())
			vol.VolumeSource.Secret.SecretName = manifestSecret.GetName()
			volumes[idx] = vol
		}
	}

	if !reflect.DeepEqual(volumes, dataGatheringJobCopy.Spec.Template.Spec.Volumes) {
		ctxlog.Debugf(ctx, "Updating ExtendedJob '%s/%s'", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName())
		err := r.client.Update(ctx, dataGatheringJob)
		if err != nil {
			ctxlog.Errorf(ctx, "Could not update ExtendedJob '%s/%s': %v", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName(), err)
			return errors.Wrapf(err, "could not update ExtendedJob '%s/%s'", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName())
		}
	}

	return nil
}
