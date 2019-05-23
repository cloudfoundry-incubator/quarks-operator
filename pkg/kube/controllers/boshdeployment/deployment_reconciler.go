package boshdeployment

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	"code.cloudfoundry.org/cf-operator/pkg/kube/apis"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/owner"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
	vss "code.cloudfoundry.org/cf-operator/pkg/kube/util/versionedsecretstore"
)

// State of instance
const (
	CreatedState              = "Created"
	UpdatedState              = "Updated"
	OpsAppliedState           = "OpsApplied"
	VariableGeneratedState    = "VariableGenerated"
	VariableInterpolatedState = "VariableInterpolated"
	DataGatheredState         = "DataGathered"
	BPMConfigsCreatedState    = "BPMConfigsCreatedState"
	DeployingState            = "Deploying"
	DeployedState             = "Deployed"
)

// Check that ReconcileBOSHDeployment implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// Owner bundles funcs to manage ownership on referenced configmaps and secrets
type Owner interface {
	RemoveOwnerReferences(context.Context, apis.Object, []apis.Object) error
	ListConfigsOwnedBy(context.Context, apis.Object) ([]apis.Object, error)
}

// NewDeploymentReconciler returns a new reconcile.Reconciler
func NewDeploymentReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver bdm.Resolver, srf setReferenceFunc, store vss.VersionedSecretStore, kubeConverter *bdm.KubeConverter) reconcile.Reconciler {

	return &ReconcileBOSHDeployment{
		ctx:                  ctx,
		config:               config,
		client:               mgr.GetClient(),
		scheme:               mgr.GetScheme(),
		resolver:             resolver,
		setReference:         srf,
		owner:                owner.NewOwner(mgr.GetClient(), mgr.GetScheme()),
		versionedSecretStore: store,
		kubeConverter:        kubeConverter,
	}
}

// ReconcileBOSHDeployment reconciles a BOSHDeployment object
type ReconcileBOSHDeployment struct {
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
	kubeConverter        *bdm.KubeConverter
}

// Reconcile starts the deployment process for a BOSHDeployment and deploys ExtendedJobs to generate required properties for instance groups and rendered BPM
func (r *ReconcileBOSHDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the BOSHDeployment instance
	instance := &bdv1.BOSHDeployment{}

	// Set the ctx to be Background, as the top-level context for incoming requests.
	ctx, cancel := context.WithTimeout(r.ctx, r.config.CtxTimeOut)
	defer cancel()

	log.Infof(ctx, "Reconciling BOSHDeployment %s", request.NamespacedName)
	err := r.client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Debug(ctx, "Skip reconcile: BOSHDeployment not found")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{},
			log.WithEvent(instance, "GetBOSHDeploymentError").Errorf(ctx, "Failed to get BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "with-ops" manifest secret
	log.Debug(ctx, "Creating with-ops manifest Secret")
	manifest, err := r.createManifestWithOps(ctx, instance)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "WithOpsManifestError").Errorf(ctx, "Failed to create with-ops manifest secret for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Convert the manifest to kube objects
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	kubeConfigs := bdm.NewKubeConfig(r.config.Namespace, manifest)
	err = kubeConfigs.Convert(*manifest)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "ManifestConversionError").Errorf(ctx, "Failed to convert BOSHDeployment '%s' to kube objects: %v", request.NamespacedName, err)
	}

	// Apply the "Variable Interpolation" ExtendedJob
	log.Debug(ctx, "Creating variable interpolation ExtendedJob")
	err = r.createVariableInterpolationEJob(ctx, instance, kubeConfigs)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "VarInterpolationError").Errorf(ctx, "Failed to create variable interpolation ExtendedJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "Data Gathering" ExtendedJob
	log.Debug(ctx, "Creating data gathering ExtendedJob")
	err = r.createDataGatheringJob(ctx, instance, kubeConfigs)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "DataGatheringError").Errorf(ctx, "Failed to create data gathering ExtendedJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "BPM Configs" ExtendedJob
	log.Debug(ctx, "Creating bpm configs ExtendedJob")
	err = r.createBPMConfigsJob(ctx, instance, manifest, *kubeConfigs)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "BPMGatheringError").Errorf(ctx, "Failed to create bpm configs ExtendedJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	return reconcile.Result{}, nil
}

// createManifestWithOps creates a secret containing the deployment manifest with ops files applied
func (r *ReconcileBOSHDeployment) createManifestWithOps(ctx context.Context, instance *bdv1.BOSHDeployment) (*bdm.Manifest, error) {
	log.Debug(ctx, "Resolving manifest")
	manifest, err := r.resolver.ResolveManifest(instance, instance.GetNamespace())
	if err != nil {
		return nil, log.WithEvent(instance, "ResolveManifestError").Errorf(ctx, "Error resolving the manifest %s: %s", instance.GetName(), err)
	}

	// Replace the name with the name of the BOSHDeployment resource
	manifest.Name = instance.GetName()

	log.Debug(ctx, "Creating manifest secret with ops")

	// Create manifest with ops as variable interpolation job input.
	manifestBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, log.WithEvent(instance, "ManifestWithOpsUnmarshalError").Errorf(ctx, "Error unmarshaling the manifest %s: %s", instance.GetName(), err)
	}

	manifestSecretName := names.CalculateSecretName(names.DeploymentSecretTypeManifestWithOps, manifest.Name, "")

	// Create a secret object for the manifest
	manifestSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      manifestSecretName,
			Namespace: instance.GetNamespace(),
		},
	}

	// Set ownership reference
	if err := r.setReference(instance, manifestSecret, r.scheme); err != nil {
		return nil, log.WithEvent(instance, "ManifestWithOpsRefError").Errorf(ctx, "Failed to set ownerReference for Secret '%s': %v", manifestSecretName, err)
	}

	// Apply the secret
	_, err = controllerutil.CreateOrUpdate(ctx, r.client, manifestSecret, func(obj runtime.Object) error {
		if s, ok := obj.(*corev1.Secret); ok {
			s.Data = map[string][]byte{}
			s.StringData = map[string]string{
				"manifest.yaml": string(manifestBytes),
			}
			return nil
		}
		return fmt.Errorf("object is not a Secret")
	})
	if err != nil {
		return nil, log.WithEvent(instance, "ManifestWithOpsApplyError").Errorf(ctx, "Failed to apply Secret '%s': %v", manifestSecretName, err)
	}

	return manifest, nil
}

// createVariableInterpolationEJob create temp manifest and variable interpolation eJob
func (r *ReconcileBOSHDeployment) createVariableInterpolationEJob(ctx context.Context, instance *bdv1.BOSHDeployment, manifest *bdm.Manifest, varIntEJob *ejv1.ExtendedJob) error {

	log.Debug(ctx, "Creating variable interpolation extendedJob")
	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, varIntEJob, r.scheme); err != nil {
		log.WithEvent(instance, "NewJobForVariableInterpolationError").Errorf(ctx, "Failed to set ownerReference for ExtendedJob '%s': %v", varIntEJob.GetName(), err)
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, varIntEJob.DeepCopy(), func(obj runtime.Object) error {
		exstEJob, ok := obj.(*ejv1.ExtendedJob)
		if !ok {
			return fmt.Errorf("object is not an ExtendedJob")
		}

		exstEJob.Labels = varIntEJob.Labels
		exstEJob.Spec = varIntEJob.Spec
		return nil
	})
	if err != nil {
		log.WarningEvent(ctx, instance, "CreateExtendedJobForVariableInterpolationError", err.Error())
		return errors.Wrapf(err, "creating or updating ExtendedJob '%s'", varIntEJob.Name)
	}

	instance.Status.State = VariableInterpolatedState

	return nil
}

// createDataGatheringJob gather data from manifest
func (r *ReconcileBOSHDeployment) createDataGatheringJob(ctx context.Context, instance *bdv1.BOSHDeployment, dataGatheringEJob *ejv1.ExtendedJob) error {
	log.Debugf(ctx, "Creating data gathering extendedJob %s/%s", dataGatheringEJob.Namespace, dataGatheringEJob.Name)

	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, dataGatheringEJob, r.scheme); err != nil {
		log.WithEvent(instance, "NewJobForDataGatheringError").Errorf(ctx, "Failed to set ownerReference for ExtendedJob '%s': %v", dataGatheringEJob.GetName(), err)
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, dataGatheringEJob.DeepCopy(), func(obj runtime.Object) error {
		exstEJob, ok := obj.(*ejv1.ExtendedJob)
		if !ok {
			return fmt.Errorf("object is not an ExtendedJob")
		}

		exstEJob.Labels = dataGatheringEJob.Labels
		exstEJob.Spec = dataGatheringEJob.Spec
		return nil
	})
	if err != nil {
		log.WarningEvent(ctx, instance, "CreateJobForDataGatheringError", err.Error())
		return errors.Wrapf(err, "creating or updating ExtendedJob '%s'", dataGatheringEJob.Name)
	}

	instance.Status.State = DataGatheredState

	return nil
}

// createBPMConfigsJob creates an ejob for creating the BPM configs from manifest
func (r *ReconcileBOSHDeployment) createBPMConfigsJob(ctx context.Context, instance *bdv1.BOSHDeployment, manifest *bdm.Manifest, kubeConfig bdm.KubeConfig) error {

	// Generate the ExtendedJob object
	bpmConfigsJob := kubeConfig.BPMConfigsJob
	log.Debugf(ctx, "Creating BPM configs extendedJob %s/%s", bpmConfigsJob.Namespace, bpmConfigsJob.Name)

	// Set BOSHDeployment instance as the owner and controller
	if err := r.setReference(instance, bpmConfigsJob, r.scheme); err != nil {
		log.WithEvent(instance, "NewJobForDataGatheringError").Errorf(ctx, "Failed to set ownerReference for ExtendedJob '%s': %v", bpmConfigsJob.GetName(), err)
		return err
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.client, bpmConfigsJob.DeepCopy(), func(obj runtime.Object) error {
		exstEJob, ok := obj.(*ejv1.ExtendedJob)
		if !ok {
			return fmt.Errorf("object is not an ExtendedJob")
		}

		exstEJob.Labels = bpmConfigsJob.Labels
		exstEJob.Spec = bpmConfigsJob.Spec
		return nil
	})
	if err != nil {
		log.WarningEvent(ctx, instance, "CreateBPMConfigsJobError", err.Error())
		return errors.Wrapf(err, "creating or updating ExtendedJob '%s'", bpmConfigsJob.Name)
	}

	instance.Status.State = BPMConfigsCreatedState

	return nil
}

