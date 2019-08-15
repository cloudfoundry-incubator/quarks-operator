package boshdeployment

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"code.cloudfoundry.org/cf-operator/pkg/bosh/converter"
	bdm "code.cloudfoundry.org/cf-operator/pkg/bosh/manifest"
	bdv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/boshdeployment/v1alpha1"
	ejv1 "code.cloudfoundry.org/cf-operator/pkg/kube/apis/extendedjob/v1alpha1"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/config"
	log "code.cloudfoundry.org/cf-operator/pkg/kube/util/ctxlog"
	"code.cloudfoundry.org/cf-operator/pkg/kube/util/names"
)

// Check that ReconcileBOSHDeployment implements the reconcile.Reconciler interface
var _ reconcile.Reconciler = &ReconcileBOSHDeployment{}

type setReferenceFunc func(owner, object metav1.Object, scheme *runtime.Scheme) error

// NewDeploymentReconciler returns a new reconcile.Reconciler
func NewDeploymentReconciler(ctx context.Context, config *config.Config, mgr manager.Manager, resolver converter.Resolver, srf setReferenceFunc) reconcile.Reconciler {

	return &ReconcileBOSHDeployment{
		ctx:          ctx,
		config:       config,
		client:       mgr.GetClient(),
		scheme:       mgr.GetScheme(),
		resolver:     resolver,
		setReference: srf,
	}
}

// ReconcileBOSHDeployment reconciles a BOSHDeployment object
type ReconcileBOSHDeployment struct {
	ctx          context.Context
	config       *config.Config
	client       client.Client
	scheme       *runtime.Scheme
	resolver     converter.Resolver
	setReference setReferenceFunc
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

	// Generate all the kube objects we need for the manifest
	log.Debug(ctx, "Converting bosh manifest to kube objects")
	jobFactory := converter.NewJobFactory(*manifest, instance.GetNamespace())

	// Apply the "Variable Interpolation" ExtendedJob
	eJob, err := jobFactory.VariableInterpolationJob()
	if err != nil {
		return reconcile.Result{}, log.WithEvent(instance, "VariableGenerationError").Errorf(ctx, "Failed to build variable interpolation eJob: %v", err)
	}

	log.Debug(ctx, "Creating variable interpolation ExtendedJob")
	err = r.createEJob(ctx, instance, eJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "VarInterpolationError").Errorf(ctx, "Failed to create variable interpolation ExtendedJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "Data Gathering" ExtendedJob
	eJob, err = jobFactory.InstanceGroupManifestJob()
	if err != nil {
		return reconcile.Result{}, log.WithEvent(instance, "InstanceGroupManifestError").Errorf(ctx, "Failed to build data gathering eJob: %v", err)

	}
	log.Debug(ctx, "Creating data gathering ExtendedJob")
	err = r.createEJob(ctx, instance, eJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "InstanceGroupManifestError").Errorf(ctx, "Failed to create data gathering ExtendedJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	// Apply the "BPM Configs" ExtendedJob
	eJob, err = jobFactory.BPMConfigsJob()
	if err != nil {
		return reconcile.Result{}, log.WithEvent(instance, "BPMConfigsError").Errorf(ctx, "Failed to build BPM configs eJob: %v", err)

	}
	log.Debug(ctx, "Creating BPM configs ExtendedJob")
	err = r.createEJob(ctx, instance, eJob)
	if err != nil {
		return reconcile.Result{},
			log.WithEvent(instance, "BPMConfigsError").Errorf(ctx, "Failed to create BPM configs ExtendedJob for BOSHDeployment '%s': %v", request.NamespacedName, err)
	}

	return reconcile.Result{}, nil
}

// createManifestWithOps creates a secret containing the deployment manifest with ops files applied
func (r *ReconcileBOSHDeployment) createManifestWithOps(ctx context.Context, instance *bdv1.BOSHDeployment) (*bdm.Manifest, error) {
	log.Debug(ctx, "Resolving manifest")
	manifest, _, err := r.resolver.WithOpsManifest(instance, instance.GetNamespace())
	if err != nil {
		return nil, log.WithEvent(instance, "WithOpsManifestError").Errorf(ctx, "Error resolving the manifest %s: %s", instance.GetName(), err)
	}

	// Replace the name with the name of the BOSHDeployment resource
	manifest.Name = instance.GetName()

	log.Debug(ctx, "Creating manifest secret with ops")

	// Create manifest with ops as variable interpolation job input.
	manifestBytes, err := manifest.Marshal()
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
	obj := manifestSecret.DeepCopy()
	op, err := controllerutil.CreateOrUpdate(ctx, r.client, obj, func() error {
		originalManifest, ok := obj.Data["manifest.yaml"]
		// Update only when manifest has been changed
		if !ok || !reflect.DeepEqual(originalManifest, manifestBytes) {
			obj.Data = map[string][]byte{}
			obj.StringData = map[string]string{
				"manifest.yaml": string(manifestBytes),
			}
		}

		return nil
	})
	if err != nil {
		return nil, log.WithEvent(instance, "ManifestWithOpsApplyError").Errorf(ctx, "Failed to apply Secret '%s': %v", manifestSecretName, err)
	}

	log.Debugf(ctx, "ResourceReference secret '%s' has been %s", manifestSecret.Name, op)

	return manifest, nil
}

// createEJob creates a an EJob and sets ownership
func (r *ReconcileBOSHDeployment) createEJob(ctx context.Context, instance *bdv1.BOSHDeployment, eJob *ejv1.ExtendedJob) error {
	if err := r.setReference(instance, eJob, r.scheme); err != nil {
		return errors.Errorf("failed to set ownerReference for ExtendedJob '%s': %v", eJob.GetName(), err)
	}

	obj := eJob.DeepCopy()
	op, err := controllerutil.CreateOrUpdate(ctx, r.client, obj, func() error {
		if shouldEJobUpdate(obj, eJob) {
			eJob.ObjectMeta.ResourceVersion = obj.ObjectMeta.ResourceVersion
			eJob.Spec.Trigger.Strategy = obj.Spec.Trigger.Strategy
			eJob.DeepCopyInto(obj)
		}
		return nil
	})

	log.Debugf(ctx, "ExtendedJob '%s' has been %s", eJob.Name, op)

	return err
}

// shouldEJobUpdate determine if EJob should be updated
func shouldEJobUpdate(oldEJob, newEJob *ejv1.ExtendedJob) bool {
	if !reflect.DeepEqual(oldEJob.Labels, newEJob.Labels) {
		return true
	}
	if !reflect.DeepEqual(oldEJob.Annotations, newEJob.Annotations) {
		return true
	}
	if !reflect.DeepEqual(oldEJob.Spec.Output, newEJob.Spec.Output) {
		return true
	}
	if !reflect.DeepEqual(oldEJob.Spec.Template, newEJob.Spec.Template) {
		return true
	}
	if !reflect.DeepEqual(oldEJob.Spec.Trigger.PodState, newEJob.Spec.Trigger.PodState) {
		return true
	}
	if oldEJob.Spec.UpdateOnConfigChange != newEJob.Spec.UpdateOnConfigChange {
		return true
	}

	return false
}
