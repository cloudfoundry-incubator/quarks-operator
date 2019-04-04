package boshdeployment

import (
	"fmt"
	"reflect"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/context"
	store "code.cloudfoundry.org/cf-operator/pkg/kube/util/store/manifest"
)

// NewManifestReconciler returns a new Reconciler for manifest secret creation
func NewManifestReconciler(log *zap.SugaredLogger, ctrConfig *context.Config, mgr manager.Manager) reconcile.Reconciler {
	jobReconcilerLog := log.Named("boshdeployment-manifest-reconciler")
	jobReconcilerLog.Info("Creating a reconciler for manifest secret")

	manifestStore := store.NewStore(mgr.GetClient(), bdm.DesiredManifestKeyName)

	return &ReconcileManifest{
		log:           jobReconcilerLog,
		ctrConfig:     ctrConfig,
		client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		manifestStore: manifestStore,
	}
}

// ReconcileManifest reconciles an Job object
type ReconcileManifest struct {
	client        client.Client
	scheme        *runtime.Scheme
	log           *zap.SugaredLogger
	ctrConfig     *context.Config
	manifestStore store.Store
}

// Reconcile reads that state of the manifest secret
func (r *ReconcileManifest) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log.Infof("Reconciling BOSHDeployment %s for desired manifest creation", request.NamespacedName)

	// Fetch the BOSHDeployment instance
	instance := &bdv1.BOSHDeployment{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.NewBackgroundContextWithTimeout(r.ctrConfig.CtxType, r.ctrConfig.CtxTimeOut)
	defer cancel()

	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.log.Debug("Skip reconcile: BOSHDeployment not found")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		r.log.Errorf("Failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// Find data gathering exJob and change
	exJobKey := types.NamespacedName{Name: fmt.Sprintf("data-gathering-%s", instance.GetName()), Namespace: instance.GetNamespace()}
	dataGatheringJob := &ejv1.ExtendedJob{}

	err = r.client.Get(ctx, exJobKey, dataGatheringJob)
	if err != nil {
		if apierrors.IsNotFound(err) {
			r.log.Debug("Skip reconcile: data gathering job not found")
			return reconcile.Result{Requeue: true}, nil
		}
		r.log.Errorf("Failed to get data gathering job '%s': %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	// Update volumes for desired manifest secret
	_, interpolatedManifestSecretName := bdm.CalculateEJobOutputSecretPrefixAndName(bdm.DeploymentSecretTypeManifestAndVars, instance.GetName(), bdm.VarInterpolationContainerName)

	r.log.Debugf("Updating manifest secret to ExtendedJob '%s/%s' volumes", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName())
	err = r.UpdateExtendedJobVolumes(ctx, instance.GetNamespace(), dataGatheringJob, interpolatedManifestSecretName)
	if err != nil {
		r.log.Errorf("Failed to update ExtendedJob '%s/%s' Volumes: %v", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName(), err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileManifest) UpdateExtendedJobVolumes(ctx context.Context, namespace string, dataGatheringJob *ejv1.ExtendedJob, interpolatedManifestSecretName string) error {
	dataGatheringJobCopy := dataGatheringJob.DeepCopy()

	volumes := dataGatheringJob.Spec.Template.Spec.Volumes
	for idx, vol := range volumes {
		if vol.VolumeSource.Secret != nil && vol.VolumeSource.Secret.SecretName == interpolatedManifestSecretName {
			manifestSecret, err := r.manifestStore.LatestSecret(ctx, namespace, vol.VolumeSource.Secret.SecretName, dataGatheringJob.Spec.Output.SecretLabels)
			if err != nil {
				r.log.Errorf("Failed to get manifest secret for '%s': %v", interpolatedManifestSecretName, err)
				return err
			}

			r.log.Debugf("Changing manifest secret from '%s' to '%s'", vol.VolumeSource.Secret.SecretName, manifestSecret.GetName())
			vol.VolumeSource.Secret.SecretName = manifestSecret.GetName()
			volumes[idx] = vol
		}
	}

	if !reflect.DeepEqual(volumes, dataGatheringJobCopy.Spec.Template.Spec.Volumes) {
		r.log.Debugf("Updating ExtendedJob '%s/%s'", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName())
		err := r.client.Update(ctx, dataGatheringJob)
		if err != nil {
			r.log.Errorf("Failed to update ExtendedJob '%s/%s': %v", dataGatheringJob.GetNamespace(), dataGatheringJob.GetName(), err)
			return err
		}
	}

	return nil
}
